package main

import (
	"github.com/Etpmls/EM-User/database"
	"github.com/Etpmls/EM-User/proto/pb"
	"github.com/Etpmls/EM-User/service"
	em "github.com/Etpmls/Etpmls-Micro/v3"
	em_define "github.com/Etpmls/Etpmls-Micro/v3/define"
	"google.golang.org/grpc"
)

const (
	AppName    = "EM-User"
	AppVersion = "v0.0.1"
)

func main()  {
	var reg = em.Register{
		Version:                map[string]string{AppName + " Version": AppVersion},
		EnabledFeature:     []string{
			em_define.EnableValidator,
			em_define.EnableTranslate,
			em_define.EnableCircuitBreaker,
			em_define.EnableServiceDiscovery,
			em_define.EnableCaptcha,
		},
		RegisterService: func(s *grpc.Server) {
			pb.RegisterUserServer(s, &service.ServiceUser{})
		},
		OverrideInterface:         em.OverrideInterface{},
		OverrideFunction:          em.OverrideFunction{},
	}
	reg.Init()
	database.NewDatabase().Init()
	reg.Run()
}