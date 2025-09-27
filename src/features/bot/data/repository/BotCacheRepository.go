package repository

import (
	"context"
	"errors"
	"tg-downloader/ent"
	"tg-downloader/ent/dbgroup"
	"tg-downloader/src/features/bot/data/converter"
	"tg-downloader/src/features/bot/domain/entity"
)

type BotCacheRepository struct {
	database  *ent.Client
	converter *converter.DbGroupToGroupConverter
}

func NewBotCacheRepository(database *ent.Client) *BotCacheRepository {
	return &BotCacheRepository{
		database:  database,
		converter: converter.NewDbGroupToGroupConverter(),
	}
}

func (r *BotCacheRepository) GetGroup(id string) (*entity.Group, error) {
	instances, err := r.database.DbGroup.Query().Where(dbgroup.Identificator(id)).All(context.Background())

	if err != nil {
		return nil, err
	}

	if len(instances) == 0 {
		return nil, errors.New("no groups found")
	}

	codec := r.converter.Convert()

	first := instances[0]

	converted := codec.Convert(*first)

	return &converted, nil
}

func (r *BotCacheRepository) WriteGroup(group *entity.Group) error {
	codec := r.converter.Parse()
	dbGroup := codec.Convert(*group)

	_, err := r.database.DbGroup.Create().
		SetIdentificator(dbGroup.Identificator).
		SetAdminUserName(dbGroup.AdminUserName).
		Save(context.Background())

	return err
}

func (r *BotCacheRepository) GetAllGroupsByUserName(username string) ([]*entity.Group, error) {
	instances, err := r.database.DbGroup.Query().Where(dbgroup.AdminUserName(username)).All(context.Background())

	if err != nil {
		return nil, err
	}

	codec := r.converter.Convert()
	groups := make([]*entity.Group, 0, len(instances))

	for _, instance := range instances {
		converted := codec.Convert(*instance)
		groups = append(groups, &converted)
	}

	return groups, nil
}

func (r *BotCacheRepository) DeleteGroup(id string) error {
	_, err := r.database.DbGroup.Delete().Where(dbgroup.Identificator(id)).Exec(context.Background())
	return err
}
