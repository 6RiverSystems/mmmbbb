package ent

func (s *Subscription) HasFullDeadLetterConfig() bool {
	return s.MaxDeliveryAttempts != nil &&
		s.DeadLetterTopicID != nil &&
		*s.MaxDeliveryAttempts > 0
}
