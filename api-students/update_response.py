import os

response_path = r'D:\TI\SEMESTER 5\Backend Lanjut PRAK\latihan-fiber\api-students\helper\response.go'
with open(response_path, 'r', encoding='utf-8') as f:
    content = f.read()

success_cursor_code = '''
func SuccessCursor(c *fiber.Ctx, message string, data any, meta *model.CursorMeta) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": message,
		"data":    data,
		"meta":    meta,
	})
}
'''
if 'SuccessCursor' not in content:
    content += success_cursor_code

with open(response_path, 'w', encoding='utf-8') as f:
    f.write(content)
