# Kontrak API Mahasiswa (Student API)

| Metode | Endpoint | Parameter | Contoh Body Permintaan | Status Kembalian | Contoh Respons |
|---|---|---|---|---|---|
| **GET** | `/api/v1/students` | `page`, `limit`, `search`, `sort`, `order`, `is_active` | *(Kosong)* | `200 OK` | `{"success":true,"message":"...","data":[...],"meta":{...}}` |
| **GET** | `/api/v1/students/:id` | `id` (di URL) | *(Kosong)* | `200 OK`, `404 Not Found`, `400 Bad Request` | `{"success":true,"message":"...","data":{...}}` |
| **POST** | `/api/v1/students` | *(Kosong)* | `{"nim":"111","name":"Andi","grade":"A","is_active":true}` | `201 Created`, `400 Bad Request`, `422 Unprocessable Entity`, `415 Unsupported Media Type` | `{"success":true,"message":"...","data":{...}}` |
| **PUT** | `/api/v1/students/:id` | `id` (di URL) | `{"nim":"111","name":"Andi","grade":"B","is_active":false}` | `200 OK`, `400 Bad Request`, `404 Not Found`, `422 Unprocessable Entity` | `{"success":true,"message":"...","data":{...}}` |
| **PATCH**| `/api/v1/students/:id` | `id` (di URL) | `{"grade":"A+"}` | `200 OK`, `400 Bad Request`, `404 Not Found`, `422 Unprocessable Entity` | `{"success":true,"message":"...","data":{...}}` |
| **DELETE**|`/api/v1/students/:id` | `id` (di URL) | *(Kosong)* | `204 No Content`, `404 Not Found`, `400 Bad Request` | *(Tidak ada body)* |