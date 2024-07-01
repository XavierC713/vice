module github.com/mmp/vice/pkg/renderer

go 1.22.4

require (
	github.com/go-gl/gl v0.0.0-20231021071112-07e5d0ea2e71
	github.com/mmp/imgui-go/v4 v4.0.0-20220911181801-968a517f674f
	github.com/mmp/vice/pkg/log v0.0.0-00010101000000-000000000000
	github.com/mmp/vice/pkg/math v0.0.0
	github.com/mmp/vice/pkg/util v0.0.0-00010101000000-000000000000
)

require (
	github.com/iancoleman/orderedmap v0.3.0 // indirect
	github.com/inkyblackness/imgui-go/v4 v4.7.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	golang.org/x/exp v0.0.0-20240613232115-7f521ea00fb8 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)

replace github.com/mmp/vice/pkg/math => /Users/mmp/vice/pkg/math

replace github.com/mmp/vice/pkg/log => /Users/mmp/vice/pkg/log

replace github.com/mmp/vice/pkg/util => /Users/mmp/vice/pkg/util

replace github.com/mmp/vice/pkg/rand => /Users/mmp/vice/pkg/rand
