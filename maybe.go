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
