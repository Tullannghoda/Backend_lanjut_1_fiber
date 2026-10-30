package service

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"
)

type SupervisorService struct {
	repo repository.SupervisorRepository
	tx   *repository.TxManager
}

func NewSupervisorService(repo repository.SupervisorRepository, tx *repository.TxManager) *SupervisorService {
	return &SupervisorService{repo: repo, tx: tx}
}

// ---------- GET /supervisors ----------
func (s *SupervisorService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	supervisors, err := s.repo.FindAll(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "daftar dosen pembimbing berhasil diambil", supervisors)
}

// ---------- GET /supervisors/:id ----------
func (s *SupervisorService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	sp, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateSupervisorError(err)
	}
	return helper.Success(c, fiber.StatusOK, "dosen pembimbing ditemukan", sp)
}

// ---------- POST /supervisors/:id/assign ----------
func (s *SupervisorService) Assign(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	supervisorID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var req model.AssignSupervisorRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	var assigned model.Supervisor
	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.repo.WithTx(tx)

		// 1. Kunci baris mahasiswa untuk mencegah penetapan serentak ke 2 pembimbing
		if err := repo.LockStudent(ctx, req.StudentID); err != nil {
			return err
		}

		// 2. Tambah jumlah bimbingan secara atomik
		var incErr error
		assigned, incErr = repo.IncrementSupervised(ctx, supervisorID)
		if incErr != nil {
			return incErr
		}

		// 3. Catat penetapan bimbingan (UNIQUE student_id mencegah penetapan ganda)
		return repo.InsertSupervision(ctx, req.StudentID, supervisorID)
	})

	if err != nil {
		return translateSupervisorError(err)
	}

	return helper.Created(c, "berhasil menetapkan dosen pembimbing", assigned,
		"/api/v1/supervisors/"+strconv.Itoa(supervisorID))
}

// ---------- POST /supervisions/switch ----------
func (s *SupervisorService) Switch(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.SwitchSupervisorRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	if req.FromSupervisorID == req.ToSupervisorID {
		return helper.Validation(map[string]string{
			"to_supervisor_id": "tidak boleh sama dengan from_supervisor_id",
		})
	}

	var newSupervisor model.Supervisor
	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.repo.WithTx(tx)

		// 1. Kunci baris mahasiswa
		if err := repo.LockStudent(ctx, req.StudentID); err != nil {
			return err
		}

		// 2. Kunci KEDUA dosen dalam urutan ID menaik untuk mencegah deadlock
		if err := repo.LockSupervisors(ctx, req.FromSupervisorID, req.ToSupervisorID); err != nil {
			return err
		}

		// 3. Hapus bimbingan lama
		if err := repo.DeleteSupervision(ctx, req.StudentID, req.FromSupervisorID); err != nil {
			return err
		}

		// 4. Kurangi kuota lama
		if _, err := repo.DecrementSupervised(ctx, req.FromSupervisorID); err != nil {
			return err
		}

		// 5. Tambah kuota baru secara atomik
		var incErr error
		newSupervisor, incErr = repo.IncrementSupervised(ctx, req.ToSupervisorID)
		if incErr != nil {
			return incErr
		}

		// 6. Catat bimbingan baru
		return repo.InsertSupervision(ctx, req.StudentID, req.ToSupervisorID)
	})

	if err != nil {
		return translateSupervisorError(err)
	}

	return helper.Success(c, fiber.StatusOK, "berhasil mengganti dosen pembimbing", newSupervisor)
}

// ---------- PATCH /supervisors/:id ----------
func (s *SupervisorService) UpdateQuota(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var req model.UpdateSupervisorQuotaRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	updated, err := s.repo.UpdateQuota(ctx, id, req.Quota, req.Version)
	if err != nil {
		return translateSupervisorError(err)
	}

	return helper.Success(c, fiber.StatusOK, "kuota pembimbing berhasil diubah", updated)
}

func translateSupervisorError(err error) error {
	switch {
	case errors.Is(err, repository.ErrSupervisorNotFound), errors.Is(err, repository.ErrNotFound):
		return helper.NotFound("pembimbing atau mahasiswa tidak ditemukan")
	case errors.Is(err, repository.ErrSupervisorFull):
		return helper.NewError(fiber.StatusConflict, "SUPERVISOR_FULL", "kuota dosen pembimbing sudah penuh")
	case errors.Is(err, repository.ErrAlreadySupervised):
		return helper.NewError(fiber.StatusConflict, "ALREADY_SUPERVISED", "mahasiswa sudah memiliki dosen pembimbing")
	case errors.Is(err, repository.ErrNotSupervised):
		return helper.NewError(fiber.StatusConflict, "NOT_SUPERVISED", "mahasiswa tidak dibimbing oleh dosen tersebut")
	case errors.Is(err, repository.ErrVersionConflict):
		return helper.NewError(fiber.StatusConflict, "VERSION_CONFLICT", "data dosen sudah diubah pihak lain, muat ulang lalu coba lagi")
	case errors.Is(err, repository.ErrQuotaBelowFilled):
		return helper.NewError(fiber.StatusConflict, "QUOTA_BELOW_SUPERVISED", "kuota tidak boleh lebih kecil dari jumlah mahasiswa bimbingan saat ini")
	default:
		return helper.Internal(err)
	}
}
