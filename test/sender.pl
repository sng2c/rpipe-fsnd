$|++;
@cmd = qw(
ME:sid=1&cmd=OPEN&file=README.txt&seq=0&hash=2cad20c19a8eb9bb11a9f76527aec9bc&txid=exam_001
ME:sid=1&cmd=WRITE&data=SEVMTE8gV09STEQK&seq=1
ME:sid=1&cmd=CLOSE&seq=3
);

for my $l (@cmd) {
	print $l."\n";
	<STDIN>;
}
