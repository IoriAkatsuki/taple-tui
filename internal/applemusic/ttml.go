package applemusic

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

// Apple uses media-clock times on both verse and line nodes, not additive offsets.
func parseLyricsTTML(input string) (Lyrics, error) {
	result := Lyrics{Synced: true}
	if len(input) == 0 || len(input) > 2_000_000 {
		return result, errors.New("歌词文档为空或超过 2MB")
	}
	d := xml.NewDecoder(strings.NewReader(input))
	invalid := func(reason string) error {
		line, column := d.InputPos()
		return fmt.Errorf("%s（XML %d:%d）", reason, line, column)
	}
	depth, bodyDepth, lineDepth, timedSpanDepth := 0, 0, 0, 0
	root, body, closed, plain := false, false, false, false
	var text, spanText strings.Builder
	var line LyricLine
	var spanStart, spanEnd time.Duration
	lineTimed := false
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, invalid("XML 文档格式错误")
		}
		switch node := token.(type) {
		case xml.Directive:
			return result, invalid("不接受 DTD 或实体声明")
		case xml.StartElement:
			depth++
			if depth > 64 {
				return result, invalid("XML 嵌套超过 64 层")
			}
			attr := func(name string) string {
				for _, a := range node.Attr {
					if a.Name.Local == name {
						return a.Value
					}
				}
				return ""
			}
			if value := attr("timeBase"); value != "" && value != "media" {
				return result, invalid("不支持的 TTML 时间基")
			}
			if value := attr("timeContainer"); value != "" && value != "par" {
				return result, invalid("不支持的 TTML 时间容器")
			}
			if depth == 1 {
				if root || closed || node.Name.Local != "tt" {
					return result, invalid("缺少唯一的 tt 根节点")
				}
				root = true
				result.Language = attr("lang")
				plain = attr("timing") == "None"
			}
			if node.Name.Local == "body" {
				if body {
					return result, invalid("重复的 body 节点")
				}
				body, bodyDepth = true, depth
			}
			if bodyDepth > 0 && node.Name.Local == "p" {
				if lineDepth != 0 {
					return result, invalid("嵌套的歌词段落")
				}
				lineDepth = depth
				text.Reset()
				spanText.Reset()
				timedSpanDepth = 0
				line = LyricLine{}
				begin, end, duration := attr("begin"), attr("end"), attr("dur")
				lineTimed = begin != "" && (end != "" || duration != "")
				if !lineTimed {
					continue
				}
				line.Start, err = parseLyricTime(begin)
				if err != nil {
					return result, invalid("行开始时间格式不支持")
				}
				if end != "" {
					line.End, err = parseLyricTime(end)
				} else {
					var length time.Duration
					length, err = parseLyricTime(duration)
					line.End = line.Start + length
				}
				if err != nil {
					return result, invalid("行结束时间或时长格式不支持")
				}
				if line.End > 24*time.Hour {
					return result, invalid("歌词时间超过 24 小时")
				}
			} else if lineDepth > 0 && node.Name.Local == "br" {
				text.WriteByte(' ')
				if timedSpanDepth > 0 {
					spanText.WriteByte(' ')
				}
			} else if lineDepth > 0 && timedSpanDepth == 0 && node.Name.Local == "span" {
				sb, se, sd := attr("begin"), attr("end"), attr("dur")
				if sb != "" && (se != "" || sd != "") {
					startDur, err1 := parseLyricTime(sb)
					var endDur time.Duration
					var err2 error
					if se != "" {
						endDur, err2 = parseLyricTime(se)
					} else {
						var length time.Duration
						length, err2 = parseLyricTime(sd)
						endDur = startDur + length
					}
					if err1 == nil && err2 == nil && startDur >= 0 && endDur > startDur && endDur <= 24*time.Hour {
						timedSpanDepth = depth
						spanStart, spanEnd = startDur, endDur
						spanText.Reset()
					}
				}
			}
		case xml.CharData:
			if lineDepth > 0 {
				text.Write(node)
				if timedSpanDepth > 0 {
					spanText.Write(node)
				}
			} else if depth == 0 && strings.TrimSpace(string(node)) != "" {
				return result, invalid("根节点之外存在文本")
			}
		case xml.EndElement:
			if timedSpanDepth > 0 && depth == timedSpanDepth {
				if st := strings.Join(strings.Fields(spanText.String()), " "); st != "" {
					line.Spans = append(line.Spans, LyricSpan{Text: st, Start: spanStart, End: spanEnd})
				}
				timedSpanDepth = 0
				spanText.Reset()
			}
			if depth == lineDepth {
				line.Text = strings.Join(strings.Fields(text.String()), " ")
				if line.Text != "" {
					if lineTimed && line.End <= line.Start {
						return result, invalid("行结束时间必须晚于开始时间")
					}
					result.Synced = result.Synced && lineTimed
					result.Lines = append(result.Lines, line)
				}
				if len(result.Lines) > 10000 {
					return result, invalid("歌词超过 10000 行")
				}
				lineDepth = 0
				timedSpanDepth = 0
			}
			if depth == bodyDepth {
				bodyDepth = 0
			}
			depth--
			if depth == 0 {
				closed = true
			}
		}
	}
	if !root || !body || !closed || depth != 0 {
		return result, invalid("缺少完整的 tt/body 结构")
	}
	result.Synced = result.Synced && !plain && len(result.Lines) > 0
	return result, nil
}

func parseLyricTime(input string) (time.Duration, error) {
	invalid := errors.New("无效或不支持的歌词时间格式")
	decimal := func(s string) (float64, error) {
		digits, dots := 0, 0
		for _, r := range s {
			if r >= '0' && r <= '9' {
				digits++
			} else if r == '.' {
				dots++
			} else {
				return 0, invalid
			}
		}
		if digits == 0 || dots > 1 {
			return 0, invalid
		}
		return strconv.ParseFloat(s, 64)
	}
	var seconds float64
	parts := strings.Split(input, ":")
	if len(parts) == 2 || len(parts) == 3 {
		for i, part := range parts {
			n, err := decimal(part)
			if err != nil || (i < len(parts)-1 && strings.Contains(part, ".")) || (i > 0 && n >= 60) {
				return 0, invalid
			}
			seconds = seconds*60 + n
		}
	} else if len(parts) == 1 {
		matched := false
		for _, unit := range []struct {
			suffix  string
			seconds float64
		}{{"ms", .001}, {"s", 1}, {"m", 60}, {"h", 3600}} {
			if value, ok := strings.CutSuffix(input, unit.suffix); ok {
				n, err := decimal(value)
				if err != nil {
					return 0, invalid
				}
				seconds, matched = n*unit.seconds, true
				break
			}
		}
		if !matched {
			// Apple's web lyrics also use bare seconds, such as "12.345" or "0".
			var err error
			seconds, err = decimal(input)
			if err != nil {
				return 0, invalid
			}
		}
	} else {
		return 0, invalid
	}
	if math.IsInf(seconds, 0) || math.IsNaN(seconds) || seconds > 86400 {
		return 0, invalid
	}
	return time.Duration(math.Round(seconds * float64(time.Second))), nil
}
