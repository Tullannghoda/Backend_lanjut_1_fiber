package service

import (
	"errors"
	"strings"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"
)

type UserService struct {
	repo  repository.UserRepository
	perms *helper.PermissionSet
}

func NewUserService(
	repo repository.UserRepository,
	perms *helper.PermissionSet,
) *UserService {
	return &UserService{repo: repo, perms: perms}
}

// List godoc
//
// @Summary      Daftar seluruh user dengan cursor pagination
// @Description  Membutuhkan permission user:list. Mendukung pencarian dan ekspor CSV via header Accept.
// @Tags         users
// @Produce      json
// @Param        limit      query     int     false  "Jumlah data per halaman"  default(10)
// @Param        cursor     query     string  false  "Cursor halaman berikutnya"
// @Param        search     query     string  false  "Cari berdasarkan username atau email"
// @Param        is_active  query     bool    false  "Filter status aktif user"
// @Success      200        {object}  model.UserCursorPage
// @Failure      400        {object}  model.ErrorResponse  "Format cursor tidak valid"
// @Failure      401        {object}  model.ErrorResponse
// @Failure      403        {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /users [get]
func (s *UserService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	format, err := helper.Negotiate(c, helper.FormatJSON, helper.FormatCSV)
	if err != nil {
		return err
	}

	q, err := helper.ParseCursorQuery(c)
	if err != nil {
		return err
	}

	rows, err := s.repo.FindAfterCursor(ctx, q)
	if err != nil {
		return helper.Internal(err)
	}

	hasMore := len(rows) > q.Limit
	if hasMore {
		rows = rows[:q.Limit]
	}

	if format == helper.FormatCSV {
		return helper.WriteUsersCSV(c, rows)
	}

	meta := &model.CursorMeta{Limit: q.Limit, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		meta.NextCursor = helper.EncodeCursor(last.CreatedAt, last.ID)
	}

	return c.Status(fiber.StatusOK).JSON(model.UserCursorPage{
		Success: true,
		Message: "daftar user berhasil diambil",
		Data:    rows,
		Meta:    meta,
	})
}

// Get godoc
//
// @Summary      Detail satu user berdasarkan ID
// @Description  User hanya boleh melihat profilnya sendiri kecuali memiliki permission user:read:any.
// @Tags         users
// @Produce      json
// @Param        id   path      int  true  "ID user"
// @Success      200  {object}  model.WebResponse{data=model.User}
// @Failure      400  {object}  model.ErrorResponse
// @Failure      401  {object}  model.ErrorResponse
// @Failure      403  {object}  model.ErrorResponse
// @Failure      404  {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /users/{id} [get]
func (s *UserService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	if !CanAccessUser(current, id, s.perms, "user:read:any") {
		return helper.Forbidden("tidak berhak mengakses data user lain")
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateUserError(err, "user")
	}

	return helper.Ok(c, fiber.StatusOK, "user ditemukan", user)
}

// Create godoc
//
// @Summary      Membuat akun user baru (admin)
// @Description  Membutuhkan permission user:update:any.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      model.CreateUserRequest  true  "Data user baru"
// @Success      201   {object}  model.WebResponse{data=model.User}
// @Header       201   {string}  Location  "URL user yang baru dibuat"
// @Failure      400   {object}  model.ErrorResponse
// @Failure      401   {object}  model.ErrorResponse
// @Failure      403   {object}  model.ErrorResponse
// @Failure      409   {object}  model.ErrorResponse  "Username atau email sudah dipakai"
// @Failure      422   {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /users [post]
func (s *UserService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	var req model.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	hashed, err := helper.HashPassword(req.Password)
	if err != nil {
		return helper.Internal(err)
	}

	user, err := s.repo.Create(ctx, model.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashed,
		Role:     "user",
		IsActive: true,
	})
	if err != nil {
		return translateUserError(err, "user")
	}

	return helper.Created(c, "user berhasil dibuat", user, "/api/v1/users/"+strconv.Itoa(user.ID))
}

// Replace godoc
//
// @Summary      Mengganti seluruh data user
// @Description  Hanya pemilik akun atau user dengan permission user:update:any.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "ID user"
// @Param        body  body      model.ReplaceUserRequest  true  "Data lengkap user"
// @Success      200   {object}  model.WebResponse{data=model.User}
// @Failure      400   {object}  model.ErrorResponse
// @Failure      401   {object}  model.ErrorResponse
// @Failure      403   {object}  model.ErrorResponse
// @Failure      404   {object}  model.ErrorResponse
// @Failure      409   {object}  model.ErrorResponse
// @Failure      422   {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /users/{id} [put]
func (s *UserService) Replace(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	if !CanAccessUser(current, id, s.perms, "user:update:any") {
		return helper.Forbidden("tidak berhak mengubah data user lain")
	}

	var req model.ReplaceUserRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	result, err := s.repo.Update(ctx, model.User{
		ID:       id,
		Username: strings.TrimSpace(req.Username),
		Email:    strings.TrimSpace(req.Email),
		IsActive: req.IsActive,
	})
	if err != nil {
		return translateUserError(err, "user")
	}

	return helper.Ok(c, fiber.StatusOK, "user berhasil diperbarui", result)
}

// Patch godoc
//
// @Summary      Mengubah sebagian field data user
// @Description  Hanya pemilik akun atau user dengan permission user:update:any.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      int                     true  "ID user"
// @Param        body  body      model.PatchUserRequest  true  "Field user yang diubah"
// @Success      200   {object}  model.WebResponse{data=model.User}
// @Failure      400   {object}  model.ErrorResponse
// @Failure      401   {object}  model.ErrorResponse
// @Failure      403   {object}  model.ErrorResponse
// @Failure      404   {object}  model.ErrorResponse
// @Failure      409   {object}  model.ErrorResponse
// @Failure      422   {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /users/{id} [patch]
func (s *UserService) Patch(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	if !CanAccessUser(current, id, s.perms, "user:update:any") {
		return helper.Forbidden("tidak berhak mengubah data user lain")
	}

	var req model.PatchUserRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if IsEmptyPatchUser(req) {
		return helper.BadRequest("body tidak boleh kosong")
	}

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateUserError(err, "user")
	}

	user = ApplyPatchUser(user, req)

	result, err := s.repo.Update(ctx, user)
	if err != nil {
		return translateUserError(err, "user")
	}

	return helper.Ok(c, fiber.StatusOK, "user berhasil diperbarui sebagian", result)
}

// AssignRole godoc
//
// @Summary      Mengubah role milik seorang user (admin)
// @Description  Membutuhkan permission role:assign. Role yang diizinkan: admin, staff, user.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      int                      true  "ID user"
// @Param        body  body      model.AssignRoleRequest  true  "Role baru"
// @Success      200   {object}  model.WebResponse{data=model.User}
// @Failure      400   {object}  model.ErrorResponse
// @Failure      401   {object}  model.ErrorResponse
// @Failure      403   {object}  model.ErrorResponse
// @Failure      404   {object}  model.ErrorResponse
// @Failure      422   {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /users/{id}/role [patch]
func (s *UserService) AssignRole(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var req model.AssignRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	if errs := ValidateAssignRole(current, id, req, s.perms); len(errs) > 0 {
		return helper.Validation(errs)
	}

	result, err := s.repo.UpdateRole(ctx, id, strings.TrimSpace(req.Role))
	if err != nil {
		return translateUserError(err, "user")
	}

	return helper.Ok(c, fiber.StatusOK, "role user berhasil diubah", result)
}

// Delete godoc
//
// @Summary      Menghapus user (admin)
// @Description  Membutuhkan permission user:delete. Admin tidak diperbolehkan menghapus akunnya sendiri.
// @Tags         users
// @Param        id   path  int  true  "ID user"
// @Success      204  "Berhasil dihapus; tanpa body"
// @Failure      400  {object}  model.ErrorResponse
// @Failure      401  {object}  model.ErrorResponse
// @Failure      403  {object}  model.ErrorResponse
// @Failure      404  {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /users/{id} [delete]
func (s *UserService) Delete(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	if current.UserID == id {
		return helper.Forbidden("tidak boleh menghapus akun sendiri")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return translateUserError(err, "user")
	}

	return helper.NoContent(c)
}

func translateUserError(err error, entity string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound(entity + " tidak ditemukan")
	case errors.Is(err, repository.ErrDuplicate):
		return helper.Conflict("username atau email sudah dipakai")
	default:
		return helper.Internal(err)
	}
}

func ApplyPatchUser(current model.User, req model.PatchUserRequest) model.User {
	if req.Username != nil {
		current.Username = strings.TrimSpace(*req.Username)
	}
	if req.Email != nil {
		current.Email = strings.TrimSpace(*req.Email)
	}
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}
	return current
}

func IsEmptyPatchUser(req model.PatchUserRequest) bool {
	return req.Username == nil && req.Email == nil && req.IsActive == nil
}
