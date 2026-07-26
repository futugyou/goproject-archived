package util

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/robfig/cron/v3"
)

func IntervalToCron(interval string) (string, error) {
	if len(interval) < 2 {
		return "", fmt.Errorf("invalid interval: %s", interval)
	}

	unit := interval[len(interval)-1]
	valStr := interval[:len(interval)-1]
	val, err := strconv.Atoi(valStr)
	if err != nil || val <= 0 {
		return "", fmt.Errorf("invalid interval value: %s", interval)
	}

	switch unit {
	case 's':
		if val >= 60 {
			return fmt.Sprintf("*/%d * * * *", val/60), nil
		}
		return fmt.Sprintf("*/%d * * * * *", val), nil
	case 'm':
		return fmt.Sprintf("*/%d * * * *", val), nil
	case 'h':
		return fmt.Sprintf("0 */%d * * *", val), nil
	default:
		return "", fmt.Errorf("unknown interval unit: %c", unit)
	}
}

func NormalizeForComparison(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	var sb strings.Builder
	sb.Grow(len(trimmed))

	wasSpace := false

	for _, ch := range trimmed {
		if unicode.IsSpace(ch) {
			if !wasSpace {
				sb.WriteByte(' ')
				wasSpace = true
			}
		} else {
			sb.WriteRune(ch)
			wasSpace = false
		}
	}

	return sb.String()
}

func NormalizeCronExpression(expression string) string {
	expression = strings.ToLower(strings.TrimSpace(expression))
	switch expression {
	case "@hourly":
		return "0 * * * *"
	case "@daily":
		return "0 0 * * *"
	case "@weekly":
		return "0 0 * * 0"
	case "@monthly":
		return "0 0 1 * *"
	default:
		return expression
	}
}

func IsValidCronExpression(expression string) bool {
	if IsBlank(expression) {
		return false
	}

	expression = NormalizeCronExpression(expression)

	var parts = strings.Split(expression, " ")
	if len(parts) != 5 {
		return false
	}

	return IsValidCronField(parts[0], 0, 59) &&
		IsValidCronField(parts[1], 0, 23) &&
		IsValidCronField(parts[2], 1, 31) &&
		IsValidCronField(parts[3], 1, 12) &&
		IsValidCronField(parts[4], 0, 6)
}

func IsValidCronField(field string, min, max int) bool {
	if IsBlank(field) {
		return false
	}

	if field == "*" {
		return true
	}

	if field == "L" {
		return min == 1
	}

	if exact, err := strconv.Atoi(field); err == nil {
		return exact >= min && exact <= max
	}

	if strings.Contains(field, ",") {
		options := strings.Split(field, ",")
		for _, option := range options {
			if option == "" || !IsValidCronField(option, min, max) {
				return false
			}
		}
		return true
	}

	if strings.Contains(field, "/") {
		var stepParts = strings.Split(field, "/")
		if len(stepParts) != 2 {
			return false
		}

		step, err := strconv.Atoi(stepParts[1])
		if err != nil || step <= 0 {
			return false
		}

		return stepParts[0] == "*" || IsValidCronField(stepParts[0], min, max)
	}

	if strings.Contains(field, "-") {
		var rangeParts = strings.Split(field, "-")
		if len(rangeParts) != 2 {
			return false
		}
		start, err1 := strconv.Atoi(rangeParts[0])
		end, err2 := strconv.Atoi(rangeParts[1])
		if err1 != nil || err2 != nil {
			return false
		}

		return start >= min && start <= max && end >= min && end <= max
	}

	return false
}

func NormalizeExpression(expression string, time time.Time) string {
	expression = strings.ToLower(strings.TrimSpace(expression))
	normalized := expression
	switch expression {
	case "@hourly":
		normalized = "0 * * * *"
	case "@daily":
		normalized = "0 0 * * *"
	case "@weekly":
		normalized = "0 0 * * 0"
	case "@monthly":
		normalized = "0 0 1 * *"
	}

	parts := strings.Split(normalized, " ")
	dayOfMonthIndex := -1
	if len(parts) == 5 {
		dayOfMonthIndex = 2
	}
	if len(parts) == 6 {
		dayOfMonthIndex = 3
	}

	if dayOfMonthIndex >= 0 && parts[dayOfMonthIndex] == "1" {
		parts[dayOfMonthIndex] = DaysInMonth(time.Year(), time.Month())
	}

	return strings.Join(parts, " ")
}

func ParseCronExpression(spec string) (cron.Schedule, bool) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, false
	}

	// 1. 处理预定义描述符（如 @yearly, @monthly, @every 1h 等）
	if strings.HasPrefix(spec, "@") {
		parser := cron.NewParser(cron.Descriptor)
		if sched, err := parser.Parse(spec); err == nil {
			return sched, true
		}
		return nil, false
	}

	// 2. 根据空格计算段数
	fields := len(strings.Fields(spec))

	var parser cron.Parser
	switch fields {
	case 5:
		// 标准 5 段式：分 时 日 月 周
		parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	case 6:
		// 常见 6 段式：秒 分 时 日 月 周
		parser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	default:
		// 其他段数（如 7 段带年份的）当前库默认不支持，直接返回失败
		return nil, false
	}

	sched, err := parser.Parse(spec)
	if err != nil {
		return nil, false
	}

	return sched, true
}
