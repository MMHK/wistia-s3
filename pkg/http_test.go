package pkg

import (
	"testing"
	_ "wistia-s3/tests"
)

func TestHTTPService_Start(t *testing.T) {
	conf := new(Config)
	conf.MarginWithENV()

	t.Logf("%+v", conf)

	service := NewHTTP(conf)
	service.Start()

	t.Log("PASS")
}
