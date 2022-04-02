# `shardmap`

[![GoDoc](https://img.shields.io/badge/api-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/secure-for-ai/shardmap)

A simple and efficient thread-safe sharded hashmap for Go, reimplemented with generics.
This is an alternative to the standard Go map and `sync.Map`, and is optimized
for when your map needs to perform lots of concurrent reads and writes.

Inspired from [shardmap](https://github.com/tidwall/shardmap/)

The implemention is a map with a list of shard maps which is a replemention of 
[robinhood hashmap](https://github.com/tidwall/rhh)). Use
[xxh3](github.com/zeebo/xxh3) to generate internal hash key.

# Getting Started

## Installing

To start using `shardmap`, install Go and run `go get`:

```sh
$ go get -u github.com/secure-for-ai/shardmap
```

This will retrieve the library.

## Usage

The `Map[K, V]` type works similar to a standard Go map, and includes four methods:
`Set`, `Get`, `Delete`, `Len`.

```go
var m shardmap.Map[string, interface{}]
m.Init()
m.Set("Hello", "Dolly!")
val, _ := m.Get("Hello")
fmt.Printf("%v\n", val)
val, _ = m.Delete("Hello")
fmt.Printf("%v\n", val)
val, _ = m.Get("Hello")
fmt.Printf("%v\n", val)

// Output:
// Dolly!
// Dolly!
// <nil>
```

## Performance

Benchmarking conncurrent SET, GET, RANGE, and DELETE operations for 
    `sync.Map`, `map[string]interface{}`, `github.com/secure-for-ai/shardmap`. 

```
go version go1.18 linux/amd64

     number of cpus: 32
     number of keys: 1000000
            keysize: 10
        random seed: 1648927927835842345

-- sync.Map --
set: 1,000,000 ops over 32 threads in 1725ms, 579,745/sec, 1724 ns/op
get: 1,000,000 ops over 32 threads in 842ms, 1,187,972/sec, 841 ns/op
rng:       100 ops over 32 threads in 1275ms, 78/sec, 12745574 ns/op
del: 1,000,000 ops over 32 threads in 589ms, 1,696,424/sec, 589 ns/op

-- stdlib map --
set: 1,000,000 ops over 32 threads in 1064ms, 939,855/sec, 1063 ns/op
get: 1,000,000 ops over 32 threads in 76ms, 13,119,187/sec, 76 ns/op
rng:       100 ops over 32 threads in 241ms, 414/sec, 2414467 ns/op
del: 1,000,000 ops over 32 threads in 575ms, 1,740,584/sec, 574 ns/op

-- github.com/orcaman/concurrent-map --
set: 1,000,000 ops over 32 threads in 199ms, 5,018,004/sec, 199 ns/op
get: 1,000,000 ops over 32 threads in 43ms, 23,478,080/sec, 42 ns/op
rng:       100 ops over 32 threads in 6587ms, 15/sec, 65870399 ns/op
del: 1,000,000 ops over 32 threads in 110ms, 9,063,957/sec, 110 ns/op

-- github.com/tidwall/shardmap --
set: 1,000,000 ops over 32 threads in 122ms, 8,182,600/sec, 122 ns/op
get: 1,000,000 ops over 32 threads in 21ms, 47,564,163/sec, 21 ns/op
rng:       100 ops over 32 threads in 186ms, 538/sec, 1858206 ns/op
del: 1,000,000 ops over 32 threads in 48ms, 20,866,017/sec, 47 ns/op
```

## Contact

<!--Josh Baker [@tidwall](http://twitter.com/tidwall)-->

## License

`shardmap` source code is available under the MIT [License](/LICENSE).
