package internal

func PSTopicName(project, topic string) string {
	return "projects/" + project + "/topics/" + topic
}

func PSSubName(project, sub string) string {
	return "projects/" + project + "/subscriptions/" + sub
}
