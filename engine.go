package main

func (unit *Unit) Run() {
	switch unit.State {
	case Avaiable:
		err := Maybe{
			unit.GetCaptchaId,
			func() error {
				unit.Log("получаю и решаю капчу...")
				return unit.SolveCaptcha()
			},
			func() error {
				if unit.Env.WipeMode != RandomThreads {
					return nil
				}
				return unit.GetRandomThread()
			},
			unit.SendPost,
			func() error {
				msg, err := unit.HandleAnswer()
				if len(msg) != 0 {
					unit.Log(msg)
				}
				return err
			},
		}.
			Eval()
		if err == nil {
			unit.FailedRequests = 0
			return
		}
		unit.HandleError(err.(UnitError))

	case NoCookies:
		unit.Env.Limiter <- void{}
		unit.Log("сессия невалидна, получаю печенюшки...")

		unit.Proxy.UserAgent = unit.Env.RandomUserAgent()
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
		if options.InternalOptions.FilterBanned {
			unit.Log("прокси забанена, удаляю")
			return
		}
		unit.State = Avaiable

	case Failed:
		unit.FailedRequests++
		if unit.FailedRequests >= options.InternalOptions.RequestsFailedLimit {
			unit.Log("превышено число неудавшихся запросов, удаляю")
			return
		}
		unit.State = Avaiable

	case SessionFailed:
		if unit.FailedSessions >= options.InternalOptions.SessionFailedLimit {
			unit.Log("превышено число неудавшихся сессий, удаляю")
			return
		}
		unit.State = NoCookies

	case ClosedSingle:
		unit.Log("тред закрыт, не могу больше годнопостить")
		return
	}
}
