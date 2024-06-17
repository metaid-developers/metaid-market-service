package tool

import "strconv"

func StrToInt64(str string) int64 {
	data, _ := strconv.ParseInt(str, 10, 64)
	return data
}
