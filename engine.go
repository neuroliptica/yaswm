package main

func (unit *Unit) Run() {
	switch unit.State {
	case Avaiable:
		err := Maybe{
			unit.GetCaptchaId,
			unit.SolveCaptcha,
			func() error {
				if unit.Env.WipeMode != RandomThreads {
					return nil
				}
				return unit.GetRandomThread()
			},
			unit.SendPost,
			unit.HandleAnswer,
		}.
			Eval()
		if err == nil {
			// timeout maybe?
			return
		}
		unit.Logf("ошибка: %v", err)
		uerr := err.(UnitError)

		switch uerr.Code {
		case NoCookiesError:
			// check for external ip?
			unit.State = NoCookies
		case BannedError:
			unit.State = Banned
		case InternalError:
			// more proper logging
			unit.State = Failed
		}

	case NoCookies:
		unit.Env.Limiter <- void{}
		unit.Log("fetching cookies")
		cookies, err := unit.Proxy.GetCookies()
		<-unit.Env.Limiter

		if err == nil {
			unit.Cookies = cookies
			unit.Log(unit.Cookies)
			unit.State = Avaiable
			return
		}
		unit.Log(err.Error())
		// logging
		// +failed, maybe filter

	case Banned:
	case Failed:
	}
}
