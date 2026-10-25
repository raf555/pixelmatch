# pixelmatch (deprecated path)

[![Go Reference](https://pkg.go.dev/badge/github.com/raf555/pixelmatch?status.svg)](https://pkg.go.dev/github.com/raf555/pixelmatch?tab=doc)

`github.com/raf555/pixelmatch` moved to
[`github.com/raf555/pixelmatch/v7`](https://pkg.go.dev/github.com/raf555/pixelmatch/v7),
so the major version matches upstream
[mapbox/pixelmatch](https://github.com/mapbox/pixelmatch) 7.2.0. For upstream
8.x (OKLab + HyAB), use
[`github.com/raf555/pixelmatch/v8`](https://pkg.go.dev/github.com/raf555/pixelmatch/v8).

From v1.0.2 on, this package forwards each call to v7. Your code keeps
compiling, and you get v7's fixes without changing anything.

## Migrating to v7

```sh
go get github.com/raf555/pixelmatch/v7@latest
```

```diff
-import "github.com/raf555/pixelmatch"
+import "github.com/raf555/pixelmatch/v7"
```

The API matches v1 exactly. `Option` is the same type in both packages, and
the error variables hold the same values, so `errors.Is` matches across them.
You can migrate one file at a time.

## Changelog

<details>
<summary><b>v1.0.2</b></summary>

- Deprecates this module path in favor of `github.com/raf555/pixelmatch/v7`.
- Each function, option and error now forwards to v7.0.0. Results stay the
  same on amd64. On arm64, rare pixels near the threshold may change, because
  v7 stops Go from fusing multiply-adds and now matches upstream.

</details>

<details>
<summary><b>v1.0.1</b> (2026-05-17)</summary>

- Reordered the internal `Options` fields to shrink the struct.
- Added tests and README updates.

</details>

<details>
<summary><b>v1.0.0</b> (2026-05-15)</summary>

- First release: a Go port of mapbox/pixelmatch 7.2.0 with the YIQ color
  metric.

</details>

## License

ISC, same as the original mapbox/pixelmatch.
