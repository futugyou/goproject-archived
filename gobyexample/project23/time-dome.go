package main

import (
	"fmt"
	"time"
)

func main() {
	p := fmt.Println
	now := time.Now()
	p(now)
	then := time.Date(2014, 3, 3, 0, 0, 1, 500234234, time.UTC)
	p(then)
	p(now.Weekday())
	p(now.Before(then))
	diff := now.Sub(then)
	p(diff)
	p(diff.Hours())
	p(then.Add(diff))

	secs := now.Unix()
	nanos := now.UnixNano()
	millis := nanos / 1000000
	p(secs)
	p(millis)
	p(nanos)
	p(time.Unix(secs, 0))
	p(time.Unix(0, nanos))

	p(now.Format(time.RFC3339))

	t1, _ := time.Parse(
		time.RFC3339,
		"2012-11-01T22:08:41+00:00")
	p(t1)
	p(now.Format("15:04:05"))
	p(now.Format("Mon Jan _2 15:04:05 2006"))
	p(now.Format("2006-01-02T15:04:05.999999-07:00"))
	fmt.Printf("%d-%02d-%02dT%02d:%02d:%02d-00:00\n",
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second())
	ansic := "Mon Jan _2 15:04:05 2006"
	_, e := time.Parse(ansic, "8:41PM")
	p(e)
}
