
gascnet�ο�gnet��evioʵ�֣�ֻ��epoll��kqueue���˼򵥵ķ�װ���ṩ��tcp���ӿɶ���д�¼��ļ�����

gascnet��gnet��evio�Ĳ�ͬ�����ڻص��д�����eventloopʵ����id�ţ��Ա�������л����ܹ�������������

����Dial��������ĳ����ַ

����NewEngine���ڴ����¼�engine,
����WithLoops��������engine��eventloop������ÿһ��eventloop��һ��epoll��kqueue��ʵ���������ֵ����0�򴴽���eventloop��������cpu����С��0��eventloop��������cpu����ȥ��ֵ������0����ʵ�ʵ�eventloop����һ����
����WithLoops��������engine��ÿһ��eventloop�Ƿ��ռһ���̡߳�

����WithProtoAddr��������service�󶨵�ip�Ͷ˿�,service���Բ����ø�ֵ��
����WithListenbacklog��������service listeen��backlog���ȡ�
����WithReusePort��WithReuseAddr����service�󶨵ĵ�ַ�Ͷ˿��Ƿ��ռ
����WithLoadBalance����service��accept���µ����Ӻ�ĸ��ؾ������

engine�Ľӿ���
LoopNum���� ���ڻ�ȡ��ǰengine��evloop����
AddTask���ڴ���һ����ָ��loopid��ִ�к��������񡣻ص���������һ��ר�ŵ�Э���е���
AddService���ڴ���service���ص���������һ��ר�ŵ�Э���е���
StartService��������service�������serviceʹ��WithProtoAddr�����˵�ַ���ʱ��󶨸õ�ַ���ص���������һ��ר�ŵ�Э���е���
StopServiceֹͣservice�������service���˵�ַ���close���ص���������һ��ר�ŵ�Э���е���
DelService�Ƴ���service���ص���������һ��ר�ŵ�Э���е���

AddToService����������ӵ�ָ��loop�ϵ�ָ��service�ϣ���Ҫ���ڹ���������������ӡ�

