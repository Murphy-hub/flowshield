package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/cloudslit/cloudslit/casdk/examples/util"
	"net"

	"github.com/cloudslit/cloudslit/casdk/caclient"
	"github.com/cloudslit/cloudslit/casdk/keygen"
	"github.com/cloudslit/cloudslit/casdk/pkg/logger"
	"github.com/cloudslit/cloudslit/casdk/pkg/spiffe"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap/zapcore"
)

var (
	caAddr   = flag.String("ca", "https://192.168.2.80:8681", "CA Server")
	ocspAddr = flag.String("ocsp", "http://192.168.2.80:8682", "Ocsp Server")
	addr     = flag.String("addr", ":6066", "")
	authKey  = "0739a645a7d6601d9d45f6b237c4edeadad904f2fce53625dfdd541ec4fc8134"
)

// go run server.go -ca https://127.0.0.1:8081 -ocsp http://127.0.0.1:8082

func init() {
	logger.GlobalConfig(logger.Conf{
		Debug: true,
		Level: zapcore.DebugLevel,
	})
}

func main() {
	flag.Parse()
	err := NewMTLSServer()
	if err != nil {
		logger.Fatal(err)
	}
	select {}
}

// NewMTLSServer mTLS Server Use example
func NewMTLSServer() error {
	l, _ := logger.NewZapLogger(&logger.Conf{
		// Level: 2,
		Level: 0,
	})
	c := caclient.NewCAI(
		caclient.WithCAServer(caclient.RoleDefault, *caAddr),
		caclient.WithOcspAddr(*ocspAddr),
		caclient.WithAuthKey(authKey),
		caclient.WithLogger(l),
		caclient.WithCSRConf(keygen.CSRConf{
			SNIHostnames: []string{"supreme"},
			IPAddresses:  []string{"10.10.10.10"},
		}),
	)
	ex, err := c.NewExchanger(&spiffe.IDGIdentity{
		SiteID:    "test_site",
		ClusterID: "cluster_test",
		UniqueID:  "server1",
	})
	if err != nil {
		return errors.Wrap(err, "Exchanger initialization failed")
	}

	// Start certificate rotation
	go ex.RotateController().Run()

	cfger, err := ex.ServerTLSConfig()
	if err != nil {
		panic(err)
	}
	cfger.BindExtraValidator(func(identity *spiffe.IDGIdentity) error {
		fmt.Println("id: ", identity)
		return nil
	})
	tlsCfg := cfger.TLSConfig()
	tlsCfg.VerifyConnection = func(state tls.ConnectionState) error {
		fmt.Println("test state connection")
		return nil
	}
	go func() {
		httpsServer(tlsCfg)
	}()
	util.ExtractCertFromExchanger(ex)
	return nil
}

func httpsServer(cfg *tls.Config) {
	ln, err := net.Listen("tcp4", *addr)
	if err != nil {
		panic(err)
	}

	defer ln.Close()

	lnTLS := tls.NewListener(ln, cfg)

	if err := fasthttp.Serve(lnTLS, func(ctx *fasthttp.RequestCtx) {
		str := ctx.Request.String()
		logger.Info("Recv: ", str)
		ctx.SetStatusCode(200)
		ctx.SetBody([]byte("Hello " + str))
	}); err != nil {
		panic(err)
	}
}
