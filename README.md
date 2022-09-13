
gascnet参考gnet和evio实现，只对epoll和kqueue做了简单的封装，提供对tcp连接可读可写事件的监听。

gascnet与gnet和evio的不同点是在回调中传递了eventloop实例的id号，以便调用者有机会能够减少锁操作。


函数说明：

函数Dial用于连接某个地址。

函数NewEngine用于创建事件engine。

函数WithLoops用于设置engine的eventloop个数，每一个eventloop是一个epoll或kqueue的实例。如果该值等于0则创建的eventloop数量等于cpu数，小于0则eventloop数量等于cpu数减去该值，大于0则与实际的eventloop数量一样。

函数WithLoops用于设置engine的每一个eventloop是否独占一个线程。



函数WithProtoAddr用于设置service绑定的ip和端口,service可以不设置该值。

函数WithListenbacklog用于设置service listeen的backlog长度。

函数WithReusePort和WithReuseAddr设置service绑定的地址和端口是否独占。

函数WithLoadBalance设置service对accept到新的连接后的负载均衡策略。



engine的接口中

LoopNum函数 用于获取当前engine的evloop数量。

AddTask用于创建一个在指定loopid上执行函数的任务。回调函数会在一个专门的协程中调用。

AddService用于创建service。回调函数会在一个专门的协程中调用。

StartService用于启动service，如果该service使用WithProtoAddr设置了地址则此时会绑定该地址。回调函数会在一个专门的协程中调用。

StopService停止service，如果该service绑定了地址则会close。回调函数会在一个专门的协程中调用。

DelService移除该service。回调函数会在一个专门的协程中调用。

AddToService用于添加连接到指定loop上的指定service上，主要用于管理主动发起的连接。

