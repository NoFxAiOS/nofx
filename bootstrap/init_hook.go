package bootstrap

import "nofx/config"

type InitHook func(config *config.Config) error

var InitHooks []InitHook

// RegisterInitHook 注册初始化钩子
func RegisterInitHook(hook InitHook) {
	InitHooks = append(InitHooks, hook)
}

// RunInitHooks 运行所有注册的初始化钩子
func RunInitHooks(c *config.Config) error {
	for _, hookF := range InitHooks {
		if err := hookF(c); err != nil {
			return err
		}
	}
	return nil
}
