# fsnd

## File system job queue

### Processing states maps to directories.
* ready
* doing
* done
* failed


### SEND FILE
```
PING ->
<- PONG SESSION_KEY
INFO ->
<- DATA SESSION_KEY
-> .
```

ME:sid=1&cmd=OPEN&file=HELLO.txt&seq=1&hash=2cad20c19a8eb9bb11a9f76527aec9bc
ME:sid=1&cmd=WRITE&data=SEVMTE8gV09STEQK&seq=2
ME:sid=1&cmd=CLOSE&seq=3

l <- c("a","b","c")