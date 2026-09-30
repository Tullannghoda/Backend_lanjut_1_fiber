import os

svc_path = r'D:\TI\SEMESTER 5\Backend Lanjut PRAK\latihan-fiber\api-students\app\service\user_service.go'
with open(svc_path, 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('translateError(', 'translateUserError(')
content = content.replace('func translateError(', 'func translateUserError(')
content = content.replace('ApplyPatch(', 'ApplyPatchUser(')
content = content.replace('func ApplyPatch(', 'func ApplyPatchUser(')
content = content.replace('IsEmptyPatch(', 'IsEmptyPatchUser(')
content = content.replace('func IsEmptyPatch(', 'func IsEmptyPatchUser(')

if '"strconv"' not in content:
    content = content.replace('"strings"', '"strings"\n\t"strconv"')

with open(svc_path, 'w', encoding='utf-8') as f:
    f.write(content)
