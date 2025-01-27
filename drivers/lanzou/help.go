package lanzou

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	log "github.com/sirupsen/logrus"
)

const DAY time.Duration = 84600000000000

// 解析时间
var timeSplitReg = regexp.MustCompile("([0-9.]*)\\s*([\u4e00-\u9fa5]+)")

func MustParseTime(str string) time.Time {
	lastOpTime, err := time.ParseInLocation("2006-01-02 -07", str+" +08", time.Local)
	if err != nil {
		strs := timeSplitReg.FindStringSubmatch(str)
		lastOpTime = time.Now()
		if len(strs) == 3 {
			i, _ := strconv.ParseInt(strs[1], 10, 64)
			ti := time.Duration(-i)
			switch strs[2] {
			case "秒前":
				lastOpTime = lastOpTime.Add(time.Second * ti)
			case "分钟前":
				lastOpTime = lastOpTime.Add(time.Minute * ti)
			case "小时前":
				lastOpTime = lastOpTime.Add(time.Hour * ti)
			case "天前":
				lastOpTime = lastOpTime.Add(DAY * ti)
			case "昨天":
				lastOpTime = lastOpTime.Add(-DAY)
			case "前天":
				lastOpTime = lastOpTime.Add(-DAY * 2)
			}
		}
	}
	return lastOpTime
}

// 解析大小
var sizeSplitReg = regexp.MustCompile(`(?i)([0-9.]+)\s*([bkm]+)`)

func SizeStrToInt64(size string) int64 {
	strs := sizeSplitReg.FindStringSubmatch(size)
	if len(strs) < 3 {
		return 0
	}

	s, _ := strconv.ParseFloat(strs[1], 64)
	switch strings.ToUpper(strs[2]) {
	case "B":
		return int64(s)
	case "K":
		return int64(s * (1 << 10))
	case "M":
		return int64(s * (1 << 20))
	}
	return 0
}

// 移除注释
func RemoveNotes(html []byte) []byte {
	return regexp.MustCompile(`<!--.*?-->|//.*|/\*.*?\*/`).ReplaceAll(html, []byte{})
}

var findAcwScV2Reg = regexp.MustCompile(`arg1='([0-9A-Z]+)'`)

// 在页面被过多访问或其他情况下，有时候会先返回一个加密的页面，其执行计算出一个acw_sc__v2后放入页面后再重新访问页面才能获得正常页面
// 若该页面进行了js加密，则进行解密，计算acw_sc__v2，并加入cookie
func CalcAcwScV2(html string) (string, error) {
	log.Debugln("acw_sc__v2", html)
	acwScV2s := findAcwScV2Reg.FindStringSubmatch(html)
	if len(acwScV2s) != 2 {
		return "", fmt.Errorf("无法匹配acw_sc__v2")
	}
	return HexXor(Unbox(acwScV2s[1]), "3000176000856006061501533003690027800375"), nil
}

func Unbox(hex string) string {
	var box = []int{6, 28, 34, 31, 33, 18, 30, 23, 9, 8, 19, 38, 17, 24, 0, 5, 32, 21, 10, 22, 25, 14, 15, 3, 16, 27, 13, 35, 2, 29, 11, 26, 4, 36, 1, 39, 37, 7, 20, 12}
	var newBox = make([]byte, len(hex))
	for i := 0; i < len(box); i++ {
		j := box[i]
		if len(newBox) > j {
			newBox[j] = hex[i]
		}
	}
	return string(newBox)
}

func HexXor(hex1, hex2 string) string {
	out := bytes.NewBuffer(make([]byte, len(hex1)))
	for i := 0; i < len(hex1) && i < len(hex2); i += 2 {
		v1, _ := strconv.ParseInt(hex1[i:i+2], 16, 64)
		v2, _ := strconv.ParseInt(hex2[i:i+2], 16, 64)
		out.WriteString(strconv.FormatInt(v1^v2, 16))
	}
	return out.String()
}

var findDataReg = regexp.MustCompile(`data[:\s]+({[^}]+})`)    // 查找json
var findKVReg = regexp.MustCompile(`'(.+?)':('?([^' },]*)'?)`) // 拆分kv

// 根据key查询js变量
func findJSVarFunc(key, data string) string {
	values := regexp.MustCompile(`var ` + key + ` = '(.+?)';`).FindStringSubmatch(data)
	if len(values) == 0 {
		return ""
	}
	return values[1]
}

// 解析html中的JSON
func htmlJsonToMap(html string) (map[string]string, error) {
	datas := findDataReg.FindStringSubmatch(html)
	if len(datas) != 2 {
		return nil, fmt.Errorf("not find data")
	}
	return jsonToMap(datas[1], html), nil
}

func jsonToMap(data, html string) map[string]string {
	var param = make(map[string]string)
	kvs := findKVReg.FindAllStringSubmatch(data, -1)
	for _, kv := range kvs {
		k, v := kv[1], kv[3]
		if v == "" || strings.Contains(kv[2], "'") || IsNumber(kv[2]) {
			param[k] = v
		} else {
			param[k] = findJSVarFunc(v, html)
		}
	}
	return param
}

func IsNumber(str string) bool {
	for _, s := range str {
		if !unicode.IsDigit(s) {
			return false
		}
	}
	return true
}

var findFromReg = regexp.MustCompile(`data : '(.+?)'`) // 查找from字符串

// 解析html中的form
func htmlFormToMap(html string) (map[string]string, error) {
	forms := findFromReg.FindStringSubmatch(html)
	if len(forms) != 2 {
		return nil, fmt.Errorf("not find file sgin")
	}
	return formToMap(forms[1]), nil
}

func formToMap(from string) map[string]string {
	var param = make(map[string]string)
	for _, kv := range strings.Split(from, "&") {
		kv := strings.SplitN(kv, "=", 2)[:2]
		param[kv[0]] = kv[1]
	}
	return param
}
