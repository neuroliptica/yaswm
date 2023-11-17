package main

import "time"

func (unit *Unit) Run() {
	iters := options.WipeOptions.Iters
	inf := (iters <= 0)

	defer unit.Wg.Done()

	for i := 0; inf || i < iters; {

		switch unit.State {
		case Avaiable:
			err := Maybe{
				unit.GetCaptchaId,
				func() error {
					unit.Log("получаю и решаю капчу...")
					return unit.SolveCaptcha()
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
				i++
				time.Sleep(time.Second * time.Duration(options.WipeOptions.Timeout))
				break
			}
			unit.HandleError(err.(UnitError))

		case NoCookies:
			unit.Env.Limiter <- void{}
			unit.Logf(
				"[%d/%d] сессия невалидна, получаю печенюшки...",
				unit.FailedSessions+1,
				options.InternalOptions.SessionFailedLimit,
			)
			unit.Proxy.UserAgent = unit.Env.RandomUserAgent()
			unit.Headers["User-Agent"] = unit.Proxy.UserAgent

			cookies, err := unit.Proxy.GetCookies()

			<-unit.Env.Limiter

			if err == nil {
				unit.Log("ок, сессия успешно получена")
				unit.Cookies = cookies
				unit.State = Avaiable
				unit.FailedSessions = 0
				break
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
			unit.FailedSessions++
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
}
