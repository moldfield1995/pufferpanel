package services

import (
	"github.com/pufferpanel/pufferpanel/v3/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Backup struct {
	DB *gorm.DB
}

func (bs *Backup) GetAllBackupsForServer(serverID string) ([]*models.Backup, error) {
	var records []*models.Backup
	query := bs.DB
	query = query.Where(&models.Backup{ServerID: serverID})

	err := query.Find(&records).Error
	if err != nil {
		return nil, err
	}

	return records, err
}

func (bs *Backup) GetById(id uint) (*models.Backup, error) {
	var record *models.Backup
	query := bs.DB
	query = query.Where(&models.Backup{ID: id})

	err := query.First(&record).Error
	if err != nil {
		return nil, err
	}

	return record, err
}

func (bs *Backup) Create(model *models.Backup) error {
	res := bs.DB.Create(model)

	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (bs *Backup) Update(model *models.Backup) error {
	res := bs.DB.Omit(clause.Associations).Save(model)
	return res.Error
}

func (bs *Backup) Delete(id uint) error {
	model := &models.Backup{
		ID: id,
	}

	err := bs.DB.Delete(model).Error
	if err != nil {
		return err
	}

	return nil
}
