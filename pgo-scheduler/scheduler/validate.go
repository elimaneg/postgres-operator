package scheduler

import (
	"errors"
	"fmt"
	"strings"

	cv2 "gopkg.in/robfig/cron.v2"
)

func validate(s ScheduleTemplate) error {
	if err := ValidateSchedule(s.Schedule); err != nil {
		return err
	}

	if err := ValidateScheduleType(s.Type); err != nil {
		return err
	}

	if err := ValidateBackRestSchedule(s.Type, s.Deployment, s.Label, s.PGBackRest.Type); err != nil {
		return err
	}

	if err := ValidatePolicySchedule(s.Type, s.Policy.Name, s.Policy.Database); err != nil {
		return err
	}

	return nil
}

func ValidateSchedule(schedule string) error {
	if _, err := cv2.Parse(schedule); err != nil {
		return fmt.Errorf("%s is not a valid schedule: ", schedule)
	}
	return nil
}

func ValidateScheduleType(schedule string) error {
	scheduleTypes := []string{
		"pgbackrest",
		"pgbasebackup",
		"policy",
	}

	schedule = strings.ToLower(schedule)
	for _, scheduleType := range scheduleTypes {
		if schedule == scheduleType {
			return nil
		}
	}

	return fmt.Errorf("%s is not a valid schedule type", schedule)
}

func ValidateBackRestSchedule(scheduleType, deployment, label, backupType string) error {
	if scheduleType == "pgbackrest" {
		if deployment == "" && label == "" {
			return errors.New("Deployment or Label required for pgBackRest schedules")
		}

		if backupType == "" {
			return errors.New("Backup Type required for pgBackRest schedules")
		}

		validBackupTypes := []string{
			"full",
			"incr",
			"diff",
		}

		var valid bool
		for _, bType := range validBackupTypes {
			if backupType == bType {
				valid = true
				break
			}
		}

		if !valid {
			return fmt.Errorf("pgBackRest Backup Type invalid: %s", backupType)
		}
	}
	return nil
}

func ValidateBaseBackupSchedule(scheduleType, pvcName string) error {
	if scheduleType == "pgbasebackup" {
		if pvcName == "" {
			return errors.New("PVC Name required for pgBaseBackup schedules")
		}
	}
	return nil
}

func ValidatePolicySchedule(scheduleType, policy, database string) error {
	if scheduleType == "policy" {
		if database == "" {
			return errors.New("Database name required for policy schedules")
		}
		if policy == "" {
			return errors.New("Policy name required for policy schedules")
		}
	}
	return nil
}
