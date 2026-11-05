package proxy

import (
	"context"
	"net"
	"time"
)

type upstreamNetwork struct {
	ctx     context.Context
	timeout time.Duration
}

func (u upstreamNetwork) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(ctx, u.timeout)
	stop := context.AfterFunc(u.ctx, cancel)
	if u.ctx.Err() != nil {
		cancel()
	}
	return ctx, func() {
		stop()
		cancel()
	}
}

func (u upstreamNetwork) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	ctx, cancel := u.operationContext(ctx)
	defer cancel()
	return (&net.Dialer{}).DialContext(ctx, network, addr)
}

func (u upstreamNetwork) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	lookupCtx, cancel := u.operationContext(ctx)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(lookupCtx, name)
	if err != nil {
		return ctx, nil, err
	}
	for _, addr := range addresses {
		if ip := addr.IP.To4(); ip != nil {
			return ctx, ip, nil
		}
	}
	if len(addresses) == 0 {
		return ctx, nil, &net.DNSError{Err: "no addresses found", Name: name, IsNotFound: true}
	}
	return ctx, addresses[0].IP, nil
}
