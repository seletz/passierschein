package fake_test

import "fmt"

func fmtv(v any) string { return fmt.Sprintf("%v %+v %#v", v, v, v) }
