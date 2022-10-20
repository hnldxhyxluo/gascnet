package gascnet

import (
	"errors"
	"fmt"
	"log"
	"net"
	"syscall"
)

type tcpconn struct {
	isopen     bool
	watchread  bool //关注可读事件
	watchwrite bool //关注可写事件
	fd         int
	svrid      int
	svr        *service
	loopid     int
	ctx        interface{}
}

func (this *tcpconn) SetCtx(ctx interface{}) {
	this.ctx = ctx
}

func (this *tcpconn) GetCtx() interface{} {
	return this.ctx
}

func (this *tcpconn) GetLoopid() int {
	return this.loopid
}

func (this *tcpconn) GetSvrid() int {
	return this.svrid
}

func (this *tcpconn) Detach() error {
	return this.svr.remove(this, false)
}

func (this *tcpconn) LocalAddr() string {
	if sa, err := syscall.Getsockname(this.fd); err == nil && sa != nil {
		return transSockaddrToString(sa)
	} else {
		return ""
	}
}

func (this *tcpconn) RemoteAddr() string {
	if sa, err := syscall.Getpeername(this.fd); err == nil && sa != nil {
		return transSockaddrToString(sa)
	} else {
		return ""
	}
}

func (this *tcpconn) SetReadBuffer(bytes int) error {
	return socketSetReadBuffer(this.fd, bytes)
}

func (this *tcpconn) SetWriteBuffer(bytes int) error {
	return socketSetWriteBuffer(this.fd, bytes)
}

func (this *tcpconn) SetLinger(sec int) error {
	return socketSetLinger(this.fd, sec)
}

func (this *tcpconn) SetKeepAlivePeriod(sec int) error {
	return socketSetKeepAlive(this.fd, sec)
}

func (this *tcpconn) SetNoDelay(nodelay bool) error {
	if nodelay {
		return socketSetNoDelay(this.fd, 1)
	} else {
		return socketSetNoDelay(this.fd, 0)
	}
}

func (this *tcpconn) Close() (err error) {
	if !this.isopen {
		return
	}
	if this.svr != nil {
		this.svr.remove(this, true)
	}
	if this.fd >= 0 {
		this.isopen = false
		syscall.Close(this.fd)
		this.fd = -1
	}
	return nil
}

func (this *tcpconn) Watch(canread, canwrite bool) error {
	if this.svr == nil {
		return errors.New("no service")
	}
	return this.svr.Watch(this, canread, canwrite)
}

/*
func (this *tcpconn) Read(buf []byte) (int, error) {
	if n, err := syscall.Read(this.fd, buf); err == nil {
		if n == 0 {
			return 0, errors.New("remote closed")
		}
		return n, nil
	} else if err == syscall.EAGAIN {
		return n, nil
	} else {
		return n, err
	}
	//return syscall.Read(this.fd, buf)
}

func (this *tcpconn) Write(buf []byte) (int, error) {
	if n, err := syscall.Write(this.fd, buf); err == nil {
		return n, nil
	} else if err == syscall.EAGAIN {
		return n, nil
	} else {
		return n, err
	}
	//return syscall.Write(this.fd, buf)
}
*/

func (this *tcpconn) Read(buf []byte) (int, error) {
	if n, err := syscall.Read(this.fd, buf); n <= 0 {
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EINTR {
				return 0, nil
			}
			return 0, err
		}
		return n, nil
	} else {
		return n, err
	}
}

func (this *tcpconn) Write(buf []byte) (int, error) {
	if n, err := syscall.Write(this.fd, buf); n <= 0 {
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EINTR {
				return 0, nil
			}
			return 0, err
		}
		return n, nil
	} else {
		return n, err
	}
}

func dial(protoaddr string) (Conn, error) {
	network, ip, port, err := parseProtoAddr(protoaddr)
	if err != nil {
		log.Println("dial parseaddr err")
		return nil, err
	}
	addr, err := net.ResolveTCPAddr(network, fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		log.Println("dial ResolveTCPAddr err")
		return nil, err
	}

	var domain int
	var sa syscall.Sockaddr
	if network == "tcp6" {
		domain = syscall.AF_INET6
		sa6 := &syscall.SockaddrInet6{Port: addr.Port}
		if addr.IP != nil {
			copy(sa6.Addr[:], addr.IP)
		}
		if addr.Zone != "" {
			var iface *net.Interface
			iface, err = net.InterfaceByName(addr.Zone)
			if err != nil {
				log.Println("dial InterfaceByName err")
				return nil, err
			}
			sa6.ZoneId = uint32(iface.Index)
		}
		sa = sa6
	} else {
		domain = syscall.AF_INET
		sa4 := &syscall.SockaddrInet4{Port: addr.Port}
		if addr.IP != nil {
			if len(addr.IP) == 16 {
				copy(sa4.Addr[:], addr.IP[12:16])
			} else {
				copy(sa4.Addr[:], addr.IP)
			}
		}
		sa = sa4
	}
	syscall.ForkLock.RLock()
	fd, err := syscall.Socket(domain, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	syscall.ForkLock.RUnlock()

	if err := syscall.SetNonblock(fd, true); err != nil {
		log.Println("dial SetNonblock err")
		return nil, err
	}
	if err = syscall.Connect(fd, sa); err != nil && err != syscall.EINPROGRESS {
		log.Println("dial Connect err")
		syscall.Close(fd)
		return nil, err
	}

	return &tcpconn{isopen: true, fd: int(fd), svrid: -1, loopid: -1}, nil
}

func transSockaddrToString(sa syscall.Sockaddr) string {
	switch sa := sa.(type) {
	case *syscall.SockaddrInet4:
		return fmt.Sprintf("%d.%d.%d.%d:%d", sa.Addr[0], sa.Addr[1], sa.Addr[2], sa.Addr[3], sa.Port)
	case *syscall.SockaddrInet6:
		var ip net.IP
		copy(ip, sa.Addr[:])
		return fmt.Sprintf("%s:%d", ip.String(), sa.Port)
	default:
		return ""
	}
}
