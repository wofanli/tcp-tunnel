# tcp-tunnel
a tcp-tunnel implemented by golang. 

It supports to forward any traffic through a tcp tunnel, regardless of the original protocol (like UDP, ARP, normal IP, etc). Besides, I create an interface for the tunnel, so we can easily apply QoS onto it. 

## usage example

### create tcp tunnel

```
VM1 with ip 172.21.7.103:
root@vm1:~# ./tcptun
>>> debug warn                # set the debug level
>>> start 172.21.7.103        # the tcp tunnel daemon will listen on 172.21.7.103
>>> add t1 t2  172.21.7.102   # add a new tcp tunnel with local name t1, remote name t2, remote addr 172.21.7.102
```

```
VM2 with ip 172.21.7.102:
root@vm2:~# ./tcptun
>>> debug warn
>>> start 172.21.7.102
>>> add t2 t1  172.21.7.103
```

### verify the setup

```
Link t1 is created automatically on vm1: 
root@vm1:~# ip link show dev t1
16: t1: <POINTOPOINT,MULTICAST,NOARP,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UNKNOWN mode DEFAULT group default qlen 500
    link/none

Link t2 is created automatically on vm2: 
root@vm2:~# ip link show dev t2
16: t2: <POINTOPOINT,MULTICAST,NOARP,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UNKNOWN mode DEFAULT group default qlen 500
    link/none
    
t1, t2 is a generic linux network interface, which can be used to apply QoS, route etc. 
    
On VM1:
ip add add 1.1.1.1/32 dev lo
ip route add 1.1.1.2/32 dev t1

On VM2: 
ip add add 1.1.1.2/32 dev lo
ip route add 1.1.1.1/32 dev t2

On VM1: 
ping 1.1.1.2 -I 1.1.1.1

On VM2:
ping 1.1.1.1 -I 1.1.1.2
```
