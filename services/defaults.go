package services

import "go.6river.tech/gosix/registry"

var defaultServices []mmmbbbService

func RegisterDefaultServices(services *registry.Registry) {
	for _, s := range defaultServices {
		services.AddService(wrapService(s))
	}
}
