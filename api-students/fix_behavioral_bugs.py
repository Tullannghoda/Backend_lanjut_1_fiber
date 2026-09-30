import os
import re

base_dir = r'D:\TI\SEMESTER 5\Backend Lanjut PRAK\latihan-fiber\api-students'

# Bug 4: config/app.go
app_path = os.path.join(base_dir, 'config', 'app.go')
with open(app_path, 'r', encoding='utf-8') as f:
    app_code = f.read()

new_log_block = '''if appErr.Status < fiber.StatusInternalServerError {
			logger.Warn("request_rejected",
				slog.String("request_id", requestID),
				slog.String("path", c.Path()),
				slog.String("code", appErr.Code),
				slog.Int("status", appErr.Status))
		} else {
			errMessage := ""
			if appErr.Unwrap() != nil {
				errMessage = appErr.Unwrap().Error()
			}
			logger.Error("request_failed",
				slog.String("request_id", requestID),
				slog.String("path", c.Path()),
				slog.String("code", appErr.Code),
				slog.Int("status", appErr.Status),
				slog.String("error", errMessage))
		}'''
app_code = re.sub(r'if appErr\.Status < fiber\.StatusInternalServerError \{.*?(?=return c\.Status)', new_log_block + '\n\n\t\t', app_code, flags=re.DOTALL)
with open(app_path, 'w', encoding='utf-8') as f:
    f.write(app_code)

# Bug 5: app/repository/user_repository.go
repo_path = os.path.join(base_dir, 'app', 'repository', 'user_repository.go')
with open(repo_path, 'r', encoding='utf-8') as f:
    repo_code = f.read()
repo_code = repo_code.replace('ORDER BY created_at ASC, id ASC', 'ORDER BY created_at DESC, id DESC')
with open(repo_path, 'w', encoding='utf-8') as f:
    f.write(repo_code)

# Bug 6: helper/negotiate.go
neg_path = os.path.join(base_dir, 'helper', 'negotiate.go')
with open(neg_path, 'r', encoding='utf-8') as f:
    neg_code = f.read()
neg_code = neg_code.replace('if err := writer.Error(); err != nil {', 'writer.Flush()\n\n\tif err := writer.Error(); err != nil {')
with open(neg_path, 'w', encoding='utf-8') as f:
    f.write(neg_code)

# Bug 7: helper/validator.go
val_path = os.path.join(base_dir, 'helper', 'validator.go')
with open(val_path, 'r', encoding='utf-8') as f:
    val_code = f.read()
val_code = val_code.replace('passwordStrength(fl.Field().String()) != ""', 'passwordStrength(fl.Field().String()) == ""')
with open(val_path, 'w', encoding='utf-8') as f:
    f.write(val_code)

# Bug 8 & 9: app/model/user.go
mod_path = os.path.join(base_dir, 'app', 'model', 'user.go')
with open(mod_path, 'r', encoding='utf-8') as f:
    mod_code = f.read()
mod_code = mod_code.replace('validate:"required,min=3,max=30,alphanum"', 'validate:"required,min=3,max=30,username"')
mod_code = mod_code.replace('validate:"omitnil,min=3,max=30,alphanum"', 'validate:"omitnil,min=3,max=30,username"')
mod_code = mod_code.replace('validate:"required,min=8,max=72,nospace"', 'validate:"required,max=72,strongpassword"')
with open(mod_path, 'w', encoding='utf-8') as f:
    f.write(mod_code)

print('Selesai memperbaiki 6 bug perilaku.')
