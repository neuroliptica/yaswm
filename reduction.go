package main

import (
	"errors"
	"strconv"
	"unicode"
)

const ques = 0xff

var ops = map[rune]bool{
	'+': true,
	'/': true,
	'*': true,
	'-': true,
}

type Parser []func(string) (string, string, error)

func (parser Parser) Eval(word string) ([]string, error) {
	result := []string{}
	for _, f := range parser {
		cur, next, err := f(word)
		if err != nil {
			return nil, err
		}
		word = next
		result = append(result, cur)
	}

	return result, nil
}

func Char(word string) (string, string, error) {
	xs := []rune(word)
	if len(xs) == 0 {
		return "", "", errors.New("empty word")
	}
	return string(xs[0]), string(xs[1:]), nil
}

func Op(word string) (string, string, error) {
	cur, next, err := Char(word)
	if !ops[[]rune(cur)[0]] {
		return "", "", errors.New("invalid operator")
	}
	return cur, next, err
}

func Eq(word string) (string, string, error) {
	cur, next, err := Char(word)
	if cur != "=" {
		return "", "", errors.New("invalid eq")
	}
	return cur, next, err
}

func MaybeNumber(word string) (string, string, error) {
	xs := []rune(word)

	if len(xs) == 0 {
		return "", "", errors.New("empty word")
	}

	result := []rune{}
	i := 0

	for _, r := range xs {
		if unicode.IsDigit(r) || r == '?' {
			result = append(result, r)
			i++
		} else {
			break
		}
	}

	if len(result) == 0 {
		return "", "", errors.New("invalid number")
	}

	next := ""
	if len(result) != len(xs) {
		next = string(xs[i:])
	}

	return string(result), next, nil
}

type Exp struct {
	L0, L1, R int
	Op        rune
	Word      string
}

func (exp *Exp) FormatWord() *Exp {
	exp.Word = string(Filter([]rune(exp.Word), func(r rune) bool {
		return unicode.IsDigit(r) || ops[r] || r == '?' || r == '='
	}))
	return exp
}

func (exp *Exp) Parse() error {
	parser := Parser{
		MaybeNumber,
		Op,
		MaybeNumber,
		Eq,
		MaybeNumber,
	}

	result, err := parser.Eval(exp.Word)

	if err != nil {
		return err
	}

	maybe := func(part string) int {
		i, err := strconv.Atoi(part)
		if err != nil {
			return ques
		}
		return i
	}

	exp.L0 = maybe(result[0])
	exp.Op = []rune(result[1])[0]
	exp.L1 = maybe(result[2])
	exp.R = maybe(result[4])

	return nil
}

func (exp *Exp) Eval() int {
	switch exp.Op {

	case '+':
		if exp.R == ques {
			return exp.L0 + exp.L1
		}
		if exp.L0 == ques {
			return exp.R - exp.L1
		}
		if exp.L1 == ques {
			return exp.R - exp.L0
		}

	case '-':
		exp.Op = '+'
		if exp.L1 != ques {
			exp.L1 = -exp.L1
			return exp.Eval()
		}
		return -exp.Eval()

	case '*':
		if exp.R == ques {
			return exp.L0 * exp.L1
		}
		if exp.L0 == ques && exp.L1 != 0 {
			return exp.R / exp.L1
		}
		if exp.L1 == ques && exp.L0 != 0 {
			return exp.R / exp.L0
		}

	case '/':
		exp.Op = '*'
		if exp.L1 != 0 && exp.L1 != ques {
			exp.L1 = 1 / exp.L1
		} else if exp.L1 == ques && exp.R != 0 {
			return exp.L0 / exp.R
		}
		return exp.Eval()
	}
	return 0
}

func Solve(word string) (string, error) {
	exp := Exp{
		Word: word,
	}

	err := exp.FormatWord().Parse()

	if err != nil {
		return word, err
	}

	return strconv.Itoa(exp.Eval()), nil
}
