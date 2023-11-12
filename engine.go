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
			unit.FailedRequests = 0
			// timeout maybe?
			return
		}
		unit.Logf("ошибка: %v", err)

		// handleError

	case NoCookies:
		unit.Env.Limiter <- void{}
		unit.Log("получаю печенюшки...")
		cookies, err := unit.Proxy.GetCookies()
		<-unit.Env.Limiter

		if err == nil {
			unit.Cookies = cookies
			unit.State = Avaiable
			unit.FailedSessions = 0
			return
		}
		unit.Log("ошибка получения сессии: ", err.Error())
		unit.State = SessionFailed

	case Banned:
		if *FilterBanned {
			unit.Log("прокси забанена, удаляю")
			return
		}
		unit.State = Avaiable

	case Failed:
		unit.FailedRequests++
		if unit.FailedRequests >= *RequestsFailedLimit {
			unit.Log("превышено число неудавшихся запросов, удаляю")
			return
		}
		unit.State = Avaiable

	case SessionFailed:
		if unit.FailedSessions >= *SessionsFailedLimit {
			unit.Log("превышено число неудавшихся сессий, удаляю")
			return
		}
		unit.State = NoCookies

	}
}
