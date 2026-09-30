import os

req_path = r'D:\TI\SEMESTER 5\Backend Lanjut PRAK\latihan-fiber\api-students\helper\request.go'
with open(req_path, 'a', encoding='utf-8') as f:
    f.write('\nfunc RequestID(c *fiber.Ctx) string {\n\tid, _ := c.Locals("requestid").(string)\n\treturn id\n}\n')
