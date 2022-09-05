package gascnet

type EngOption func(*engoption)

type engoption struct {
	numloops   int
	lockthread bool
}

func WithLoops(value int) EngOption {
	return func(opt *engoption) {
		opt.numloops = value
	}
}

func WithLockThread(value bool) EngOption {
	return func(opt *engoption) {
		opt.lockthread = value
	}
}

type LoadBalance int

const (
	LoadBalanceRR  LoadBalance = iota //轮询
	LoadBalanceLC                     //最少连接
	LoadBalanceSAH                    //源地址哈希
)

type SvrOption func(*svroption)

type svroption struct {
	reuseport     bool
	reuseaddr     bool
	listenbacklog int
	protoaddr     string
	lb            LoadBalance
}

func WithProtoAddr(value string) SvrOption {
	return func(opt *svroption) {
		opt.protoaddr = value
	}
}

func WithListenbacklog(value int) SvrOption {
	return func(opt *svroption) {
		opt.listenbacklog = value
	}
}

func WithReusePort(value bool) SvrOption {
	return func(opt *svroption) {
		opt.reuseport = value
	}
}

func WithReuseAddr(value bool) SvrOption {
	return func(opt *svroption) {
		opt.reuseaddr = value
	}
}

func WithLoadBalance(value LoadBalance) SvrOption {
	return func(opt *svroption) {
		opt.lb = value
	}
}

type Conn interface {
	SetCtx(ctx interface{})
	GetCtx() (ctx interface{})

	//当前连接运行的 loopid
	GetLoopid() int
	//从loop上移除
	Detach() error

	LocalAddr() string
	RemoteAddr() string

	//socket读写缓存大小
	SetReadBuffer(bytes int) error
	SetWriteBuffer(bytes int) error

	//socket opt
	SetLinger(sec int) error
	SetKeepAlivePeriod(sec int) error
	SetNoDelay(nodelay bool) error

	Close() error

	Watch(canread, canwrite bool) error

	Read(buf []byte) (int, error)
	Write(buf []byte) (int, error)
}

type EvHandler interface {
	//service出错时调用
	OnServiceErr(loopid int, err error)

	//新连接创建成功调用
	OnConnOpen(loopid int, conn Conn)

	//连接关闭成功调用
	OnConnClose(loopid int, conn Conn, err error)

	//连接接收到流量调用
	OnConnReadWrite(loopid int, conn Conn, canread, canwrite bool)
}

type Engine interface {
	LoopNum() int
	AddTask(loopid int, call func(loopid int)) error

	AddService(name string, call func(name string, err error), evhandle EvHandler, opts ...SvrOption) error
	StartService(name string, call func(name string, err error)) error
	StopService(name string, call func(name string, err error)) error
	DelService(name string, call func(name string, err error)) error

	//not thread safe
	AddToService(name string, loopid int, conn Conn) error
}

func Dial(protoaddr string) (Conn, error) {
	return dial(protoaddr)
}

func NewEngine(opts ...EngOption) Engine {
	return newEngine(opts...)
}
