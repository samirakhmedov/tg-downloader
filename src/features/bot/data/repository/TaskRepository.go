package repository

import (
	"context"
	"tg-downloader/ent"
	"tg-downloader/ent/task"
	"tg-downloader/src/features/bot/data/converter"
	"tg-downloader/src/features/bot/domain/entity"
	"tg-downloader/src/features/bot/domain/repository"
)

type TaskRepository struct {
	database  *ent.Client
	converter *converter.TaskToDbTaskConverter
}

func NewTaskRepository(database *ent.Client) repository.ITaskRepository {
	return &TaskRepository{
		database:  database,
		converter: converter.NewTaskToDbTaskConverter(),
	}
}

func (r *TaskRepository) CreateTask(link string, groupID int64, messageID int) (*entity.Task, error) {
	// Check if task with this link already exists
	existingTask, err := r.FindTaskByLink(link)
	if err == nil {
		// Task exists, add group to it
		err = r.AddGroupToTask(existingTask.ID, groupID, messageID)
		if err != nil {
			return nil, err
		}
		// Return updated task
		return r.FindTaskByLink(link)
	}

	// Create new task with statusMessageIDs map
	statusMessageIDs := map[int64]int{groupID: messageID}

	dbTask, err := r.database.Task.Create().
		SetLink(link).
		SetGroupIDs([]int64{groupID}).
		SetStatusMessageIDs(statusMessageIDs).
		SetStatus(string(entity.TaskStatusPending)).
		Save(context.Background())

	if err != nil {
		return nil, err
	}

	codec := r.converter.Convert()
	domainTask := codec.Convert(*dbTask)
	return &domainTask, nil
}

func (r *TaskRepository) GetNextTask() (*entity.Task, error) {
	dbTask, err := r.database.Task.Query().
		Where(task.Status(string(entity.TaskStatusPending))).
		Order(ent.Asc(task.FieldID)).
		First(context.Background())

	if err != nil {
		return nil, err
	}

	codec := r.converter.Convert()
	domainTask := codec.Convert(*dbTask)
	return &domainTask, nil
}

func (r *TaskRepository) MarkTaskInProgress(id int) error {
	_, err := r.database.Task.UpdateOneID(id).
		SetStatus(string(entity.TaskStatusInProgress)).
		Save(context.Background())
	return err
}

func (r *TaskRepository) DeleteTask(id int) error {
	_, err := r.database.Task.Delete().
		Where(task.ID(id)).
		Exec(context.Background())
	return err
}

func (r *TaskRepository) FindTaskByLink(link string) (*entity.Task, error) {
	dbTask, err := r.database.Task.Query().
		Where(task.Link(link)).
		First(context.Background())

	if err != nil {
		return nil, err
	}

	codec := r.converter.Convert()
	domainTask := codec.Convert(*dbTask)
	return &domainTask, nil
}

func (r *TaskRepository) AddGroupToTask(taskID int, groupID int64, messageID int) error {
	// Get current task
	dbTask, err := r.database.Task.Get(context.Background(), taskID)
	if err != nil {
		return err
	}

	// Check if group already exists
	for _, existingGroupID := range dbTask.GroupIDs {
		if existingGroupID == groupID {
			return nil // Group already exists, no need to add
		}
	}

	// Add new group ID
	updatedGroupIDs := append(dbTask.GroupIDs, groupID)

	// Update statusMessageIDs map
	updatedStatusMessageIDs := dbTask.StatusMessageIDs
	if updatedStatusMessageIDs == nil {
		updatedStatusMessageIDs = make(map[int64]int)
	}
	updatedStatusMessageIDs[groupID] = messageID

	// Update task with new group IDs and statusMessageIDs
	_, err = r.database.Task.UpdateOneID(taskID).
		SetGroupIDs(updatedGroupIDs).
		SetStatusMessageIDs(updatedStatusMessageIDs).
		Save(context.Background())

	return err
}