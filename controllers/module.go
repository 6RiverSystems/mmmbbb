// Copyright (c) 2024 6 River Systems
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package controllers

import (
	"go.uber.org/fx"

	"go.6river.tech/mmmbbb/faults"
	"go.6river.tech/mmmbbb/services"
)

var Module = fx.Module(
	"controllers",
	// TODO: providers instead of suppliers?
	fx.Provide(fx.Annotate(
		func(readies []services.ReadyCheck) *UptimeController {
			return &UptimeController{readies: readies}
		},
		fx.ParamTags(services.ReadyTag),
		fx.As(new(Controller)),
		fx.ResultTags(ControllersTag),
	)),
	SupplyController(&LogController{}),
	fx.Provide(fx.Annotate(
		func(shutdowner fx.Shutdowner) *KillController {
			return &KillController{shutdowner: shutdowner}
		},
		fx.As(new(Controller)),
		fx.ResultTags(ControllersTag),
	)),

	SupplyController(&DelayInjectorController{}),
	fx.Provide(fx.Annotate(
		func(faults *faults.Set) *FaultInjectorController {
			return &FaultInjectorController{
				faults: faults,
			}
		},
		fx.As(new(Controller)),
		fx.ResultTags(ControllersTag),
	)),
)

const ControllersTag = `group:"controllers"`

func SupplyController(c Controller) fx.Option {
	return fx.Supply(fx.Annotate(c, fx.As(new(Controller)), fx.ResultTags(ControllersTag)))
}
