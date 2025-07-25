package main

import (
	"time"

	"github.com/rs/zerolog/log"
)

func (unit *Unit) Run() {
	iters := options.WipeOptions.Iters
	inf := (iters <= 0)

	defer unit.Wg.Done()

	for i := 0; inf || i < iters; {

		switch unit.State {
		case Avaiable:
			err := Maybe{
				unit.WithStageInfo(unit.GetCaptchaId, CaptchaId),
				unit.WithStageInfo(unit.GetCaptchaImage, CaptchaGet),
				func() error {
					for i := 0; i < 3; i++ {
						err := unit.WithStageInfo(unit.ClickCaptcha, CaptchaClick)()
						if err != nil {
							return err
						}
					}
					return nil
				},
				//func() error {
				//	unit.Log("получаю и решаю капчу...")
				//	return unit.SolveCaptcha()
				//},
				unit.WithStageInfo(unit.SendPost, SendingPost),
				func() error {
					msg, err := unit.HandleAnswer()
					if len(msg) != 0 {
						log.Info().Msgf(
							"%s -> %s",
							unit.Proxy.String(),
							msg,
						)
					}
					return err
				},
			}.
				Eval()

			if err == nil {
				unit.FailedRequests = 0
				i++
			} else {
				unit.HandleError(err.(UnitError))
			}
			time.Sleep(time.Second * time.Duration(options.WipeOptions.Timeout))

		case NoCookies:
			unit.Env.Limiter <- void{}
			log.Info().Fields(map[string]interface{}{
				"failed": unit.FailedSessions,
				"limit":  options.InternalOptions.SessionFailedLimit,
			}).Msgf("%s -> получаю печенюшки", unit.Proxy.String())

			unit.Proxy.UserAgent = unit.Env.RandomUserAgent()
			unit.Headers["User-Agent"] = unit.Proxy.UserAgent
			cookies, err := unit.Proxy.GetCookies()

			<-unit.Env.Limiter

			if err == nil {
				log.Info().Msgf("%s -> ок, сессия получена", unit.Proxy.String())
				unit.Cookies = cookies
				unit.State = Avaiable
				unit.FailedSessions = 0
				break
			}
			log.Error().Fields(map[string]interface{}{
				"err": err.Error(),
			}).Msgf("%s -> ошибка получения сесссии", unit.Proxy.String())
			unit.State = SessionFailed

		case Banned:
			if options.InternalOptions.FilterBanned {
				log.Warn().Msgf("%s -> прокси забанена, удаляю", unit.Proxy.String())
				return
			}
			unit.State = Avaiable

		case Failed:
			unit.FailedRequests++
			if unit.FailedRequests >= options.InternalOptions.RequestsFailedLimit {
				log.Warn().Msgf(
					"%s -> превышено число неудавшихся запросов, удаляю",
					unit.Proxy.String(),
				)
				return
			}
			unit.State = Avaiable

		case SessionFailed:
			unit.FailedSessions++
			if unit.FailedSessions >= options.InternalOptions.SessionFailedLimit {
				log.Warn().Msgf(
					"%s -> превышено число неудавшихся сессий, удаляю",
					unit.Proxy.String(),
				)
				return
			}
			unit.State = NoCookies

		case ClosedSingle:
			log.Warn().Msgf(
				"%s -> тред закрыт, не могу больше годнопостить",
				unit.Proxy.String(),
			)
			return
		}
	}
}
