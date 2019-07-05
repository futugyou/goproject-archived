package main

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type Nested struct {
	Email string `validate:"email"`
}

type T struct {
	Age    int `validate:"eq=10"`
	Nested Nested
}

func validateEmail(input string) bool {
	if pass, _ := regexp.MatchString(`^([\w\.\_]{2,10})@(\w{1,}).([a-z]{2,4})$`, input); pass {
		return true
	}
	return false
}

func validate(v interface{}) (bool, string) {
	validateResult := true
	errmsg := "success"
	vt := reflect.TypeOf(v)
	vv := reflect.ValueOf(v)
	for i := 0; i < vv.NumField(); i++ {
		filedVal := vv.Field(i)
		tagContent := vt.Field(i).Tag.Get("validate")
		k := filedVal.Kind()

		switch k {
		case reflect.Int:
			val := filedVal.Int()
			tagValStr := strings.Split(tagContent, "=")
			tagVal, _ := strconv.ParseInt(tagValStr[1], 10, 64)
			if val != tagVal {
				errmsg = "validate int failed, tag is: " + strconv.FormatInt(tagVal, 10)
				validateResult = false
			}

		case reflect.String:
			val := filedVal.String()
			tagValStr := tagContent
			switch tagValStr {
			case "email":
				nestedResult := validateEmail(val)
				if nestedResult == false {
					errmsg = "validate mail failed, field val is: " + val
					validateResult = false
				}
			}
		case reflect.Struct:
			valInter := filedVal.Interface()
			nestedReslut, msg := validate(valInter)
			if nestedReslut == false {
				validateResult = false
				errmsg = msg
			}

		}
	}
	return validateResult, errmsg
}

func main() {
	var a = T{Age: 11, Nested: Nested{Email: "abc@abc.com"}}

	validateResult, errmsg := validate(a)
	fmt.Println(validateResult, errmsg)
}
