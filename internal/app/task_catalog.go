package app

import taskmodule "github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/task"

var runtimeTaskCatalog = mustTaskCatalog(
	taskmodule.TaskRegistration{TaskType: taskmodule.SystemTestTaskType, Handler: taskmodule.SystemTestHandler{}},
)

func mustTaskCatalog(registrations ...taskmodule.TaskRegistration) *taskmodule.TaskCatalog {
	catalog, err := taskmodule.NewTaskCatalog(registrations...)
	if err != nil {
		panic(err)
	}
	return catalog
}

func RuntimeTaskTypes() []string {
	return runtimeTaskCatalog.Types()
}

func newTaskRegistry(catalog *taskmodule.TaskCatalog) (*taskmodule.Registry, error) {
	registry := taskmodule.NewRegistry()
	if err := catalog.RegisterHandlers(registry); err != nil {
		return nil, err
	}
	return registry, nil
}
