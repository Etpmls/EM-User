package client

import (
	"context"
	"github.com/Etpmls/EM-Auth/src/application/protobuf"
	"github.com/Etpmls/EM-User/proto/pb"
	em "github.com/Etpmls/Etpmls-Micro/v3"
	"github.com/Etpmls/Etpmls-Micro/v3/proto/empb"
)

var (
	serviceName = "AttachmentRpcService"
)

func (this *client) AttachmentDelete(ctx *context.Context, owner_ids []uint32, owner_type string) error {
	cl := em.NewClient()
	err := cl.ConnectServiceWithToken(serviceName, ctx)
	if err != nil {
		return err
	}

	c := pb.NewAttachmentClient(cl.Conn)
	return cl.Sync(func() error {
		_, err := c.Delete(*ctx, &pb.AttachmentDelete{
			Service:       em.Micro.Config.Service.RpcName,
			OwnerIds:       owner_ids,
			OwnerType:     owner_type,
		})
		return err
	}, nil)

}

func (this *client) AttachmentCreateMany(ctx *context.Context, paths []string, owner_id uint32, owner_type string) error {
	cl := em.NewClient()
	err := cl.ConnectServiceWithToken(serviceName, ctx)
	if err != nil {
		return err
	}

	c := pb.NewAttachmentClient(cl.Conn)

	return cl.Sync(func() error {
		_, err := c.CreateMany(*ctx, &pb.AttachmentCreateMany{
			Service:       em.Micro.Config.Service.RpcName,
			Paths:         paths,
			OwnerId:       owner_id,
			OwnerType:     owner_type,
		})
		return err
	}, nil)
}

func (this *client) AttachmentGetMany(ctx *context.Context, owner_ids []uint32, owner_type string) ([]byte, error) {
	cl := em.NewClient()
	err := cl.ConnectServiceWithToken(serviceName, ctx)
	if err != nil {
		return nil, err
	}

	c := pb.NewAttachmentClient(cl.Conn)
	var b []byte
	err = cl.Sync(func() error {
		r, err := c.GetMany(*ctx, &pb.AttachmentGetMany{
			Service:       em.Micro.Config.Service.RpcName,
			OwnerIds:       owner_ids,
			OwnerType:     owner_type,
		})

		if r != nil {
			b = []byte(r.GetData())
		}

		return err
	}, nil)

	return b, err
}

func (this *client) AttachmentAppend(ctx *context.Context, paths []string, owner_id uint32, owner_type string, cb func(error) error) ([]byte, error) {
	cl := em.NewClient()
	err := cl.ConnectServiceWithToken(serviceName, ctx)
	if err != nil {
		em.LogWarn.Path(err)
		return nil, err
	}

	c := pb.NewAttachmentClient(cl.Conn)
	return cl.Sync_SimpleV2(func() (response *empb.Response, e error) {
		return c.Append(*ctx, &protobuf.AttachmentAppend{
			Service:       em.Micro.Config.Service.RpcName,
			Paths:         paths,
			OwnerId:       owner_id,
			OwnerType:     owner_type,
		})
	},cb)
}