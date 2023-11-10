package main

type Maybe []func() error

func (m Maybe) Eval() error {
	for _, f := range m {
		err := f()
		if err != nil {
			return err
		}
	}
	return nil
}

func All[T any](list []T, p func(T) bool) bool {
	for i := range list {
		if !p(list[i]) {
			return false
		}
	}
	return true
}

func Any[T any](list []T, p func(T) bool) bool {
	return !All(list, func(x T) bool {
		return !p(x)
	})
}

func Map[A any, B any](list []A, f func(A) B) []B {
	l := make([]B, 0)
	for i := range list {
		l = append(l, f(list[i]))
	}
	return l
}

func Filter[A any](list []A, f func(A) bool) []A {
	l := make([]A, 0)
	for i := range list {
		if f(list[i]) {
			l = append(l, list[i])
		}
	}
	return l
}
