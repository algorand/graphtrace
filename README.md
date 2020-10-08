# graphtrace tool for tracing message propagation around a software mesh network

## messages

epoch start is 2020-01-01 00:00:00 UTC

time is varuint microseconds since epoch

Ping - acheive Network Time Protocol
0x01
sender's time
previous message's time or 0

Trace
0x02
sender's time
message id: varuint length followed by bytes; effectively length-prefixed []byte
