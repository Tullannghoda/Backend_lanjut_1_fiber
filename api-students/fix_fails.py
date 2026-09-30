import os
import re

auth_path = r'D:\TI\SEMESTER 5\Backend Lanjut PRAK\latihan-fiber\api-students\app\service\auth_service.go'
with open(auth_path, 'r', encoding='utf-8') as f:
    auth = f.read()

auth = auth.replace('helper.Fail(c, fiber.StatusBadRequest, ', 'helper.BadRequest(')
auth = auth.replace('helper.FailValidation(c, errs)', 'helper.Validation(errs)')
auth = auth.replace('helper.Fail(c, fiber.StatusInternalServerError, ', 'helper.Internal(errors.New(')
auth = auth.replace('helper.Fail(c, fiber.StatusConflict, ', 'helper.Conflict(')
auth = auth.replace('helper.Fail(c, fiber.StatusUnauthorized, ', 'helper.Unauthorized(')
auth = auth.replace('helper.Fail(c, fiber.StatusForbidden, ', 'helper.Forbidden(')

auth = re.sub(r'helper\.Internal\(errors\.New\((.*?)\)', r'helper.Internal(errors.New(\1))', auth)

with open(auth_path, 'w', encoding='utf-8') as f:
    f.write(auth)

route_path = r'D:\TI\SEMESTER 5\Backend Lanjut PRAK\latihan-fiber\api-students\route\route.go'
with open(route_path, 'r', encoding='utf-8') as f:
    rt = f.read()
rt = rt.replace('return helper.Fail(c, fiber.StatusServiceUnavailable, "database tidak dapat dihubungi")', 'return &helper.AppError{Code: "database_down", Status: fiber.StatusServiceUnavailable, Message: "database tidak dapat dihubungi"}')
with open(route_path, 'w', encoding='utf-8') as f:
    f.write(rt)
