# Quickstart: Questions CSV Import

**Feature**: 004-questions-csv-import  
**Date**: 2026-05-03

Prerequisites: Docker Compose running (`docker compose up -d`), admin token obtained via `POST /api/v1/auth/login`.

---

## 1. Download the CSV template

```bash
curl -o template.csv http://localhost:3000/api/v1/questions/import/template
cat template.csv
# Expected output:
# text,option_a,option_b,option_c,option_d,correct_index,category_name,difficulty
```

No auth required. Verify the file has exactly the 8 column headers and no data rows.

---

## 2. Prepare a sample CSV

Create `sample.csv` with at least one valid row and one invalid row. Replace `Storia` with a category name that exists in your database.

```
text,option_a,option_b,option_c,option_d,correct_index,category_name,difficulty
"In che anno cadde l'Impero Romano d'Occidente?","476 d.C.","410 d.C.","395 d.C.","455 d.C.",0,Storia,medium
"Chi dipinse la Cappella Sistina?","Leonardo","Michelangelo","Raffaello","Botticelli",1,Arte,easy
"Riga con categoria inesistente","A","B","C","D",0,Categoria_che_non_esiste,hard
```

---

## 3. Import in abort mode (default)

```bash
TOKEN="<your-admin-jwt>"

curl -s -X POST http://localhost:3000/api/v1/questions/import \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@sample.csv" \
  -F "on_error=abort" | jq .
```

**Expected result** (one invalid row → nothing imported):
```json
{
  "data": {
    "imported": 0,
    "skipped": 0,
    "errors": [
      { "row": 3, "message": "category 'Categoria_che_non_esiste' not found" }
    ]
  },
  "error": null
}
```

---

## 4. Import in skip mode

```bash
curl -s -X POST http://localhost:3000/api/v1/questions/import \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@sample.csv" \
  -F "on_error=skip" | jq .
```

**Expected result** (2 valid rows imported, 1 invalid row skipped):
```json
{
  "data": {
    "imported": 2,
    "skipped": 1,
    "errors": [
      { "row": 3, "message": "category 'Categoria_che_non_esiste' not found" }
    ]
  },
  "error": null
}
```

Verify by listing questions: `GET /api/v1/questions` — should show the 2 new questions.

---

## 5. Test row limit enforcement

Create a CSV with 501 data rows (easily done with a script):

```bash
python3 -c "
print('text,option_a,option_b,option_c,option_d,correct_index,category_name,difficulty')
for i in range(501):
    print(f'\"Q{i}\",\"A\",\"B\",\"C\",\"D\",0,Storia,medium')
" > big.csv

curl -s -X POST http://localhost:3000/api/v1/questions/import \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@big.csv" | jq .
```

**Expected**: 422 with `"message": "file contains 501 rows; maximum is 500"`.

---

## 6. Test file size limit

```bash
# Generate a file > 5MB
python3 -c "
print('text,option_a,option_b,option_c,option_d,correct_index,category_name,difficulty')
for i in range(100):
    print('\"' + 'x' * 60000 + f' Q{i}\",\"A\",\"B\",\"C\",\"D\",0,Storia,medium')
" > toobig.csv

curl -s -X POST http://localhost:3000/api/v1/questions/import \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@toobig.csv" | jq .
```

**Expected**: 422 with file size error.

---

## 7. Test unauthorized access

```bash
curl -s -X POST http://localhost:3000/api/v1/questions/import \
  -F "file=@sample.csv" | jq .
```

**Expected**: 401 Unauthorized.

---

## 8. Test duplicate warning

Upload the same `sample.csv` (skip mode, valid rows only) a second time. The second import should return warnings for duplicate rows but still succeed (imported count matches valid rows).
