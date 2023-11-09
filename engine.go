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
	case Banned:
	case Failed:
	}
}
