# manyheads

gets many HEADS

It also does GETs though, so...

I needed this to weed out the non-websites from a truly massive output from Sublist3r.
It's hastily thrown together, but it worked for me :)

## Usage

Feed it a file of domains to start sending requests.
It will automatically pull from domains.txt if not provided an argument)

example:

``` txt
$ mheads exampledomains.txt
Sending:  HEAD https://example.com
Sending:  HEAD http://example.com


HEAD http://example.com [SUCCESS]
--------
HTTP/1.1 200 OK
Content-Length: 606
Accept-Ranges: bytes
Cache-Control: max-age=604800
Content-Encoding: gzip
Content-Type: text/html; charset=UTF-8
Date: Fri, 07 Sep 2018 04:57:39 GMT
Etag: "1541025663+gzip"
Expires: Fri, 14 Sep 2018 04:57:39 GMT
Last-Modified: Fri, 09 Aug 2013 23:54:35 GMT
Server: ECS (sjc/4FC1)
X-Cache: HIT


HEAD https://example.com [SUCCESS]
--------
HTTP/2.0 200 OK
Content-Length: 606
Accept-Ranges: bytes
Cache-Control: max-age=604800
Content-Encoding: gzip
Content-Type: text/html; charset=UTF-8
Date: Fri, 07 Sep 2018 04:57:40 GMT
Etag: "1541025663"
Expires: Fri, 14 Sep 2018 04:57:40 GMT
Last-Modified: Fri, 09 Aug 2013 23:54:35 GMT
Server: ECS (sjc/4F91)
X-Cache: HIT
```

helptext:

``` txt
Usage of ./mheads:
  -agent string
    	user agent string (default "Mozilla/4.0 (compatible; MSIE 6.0; Windows NT 5.1; FSL 7.0.6.01001)")
  -json
    	true to output in json
  -method string
    	valid HTTP method (default "HEAD")
  -workers int
    	concurrent workers to spawn (default 10)
```