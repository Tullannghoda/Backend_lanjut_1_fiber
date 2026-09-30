import os

req_path = r'D:\TI\SEMESTER 5\Backend Lanjut PRAK\latihan-fiber\api-students\helper\request.go'
with open(req_path, 'r', encoding='utf-8') as f:
    content = f.read()

cursor_query_code = '''
func ParseCursorQuery(c *fiber.Ctx) (model.CursorQuery, error) {
	q := model.CursorQuery{
		Limit: c.QueryInt("limit", 10),
	}

	search := c.Query("search")
	if search != "" {
		q.Search = search
	}

	if c.Query("is_active") != "" {
		isActive := c.QueryBool("is_active")
		q.IsActive = &isActive
	}

	cursor := c.Query("cursor")
	if cursor != "" {
		decoded, err := DecodeCursor(cursor)
		if err != nil {
			return q, BadRequest("format cursor tidak valid")
		}
		q.After = &decoded
	}

	return q, nil
}
'''
if 'ParseCursorQuery' not in content:
    content += cursor_query_code

with open(req_path, 'w', encoding='utf-8') as f:
    f.write(content)
