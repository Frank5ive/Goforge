# GoForge benchmark results

Model: sonnet. Values are means per run unless noted.

| Task | Condition | Runs | Checks passed | Fully correct | Cost | Tokens | Turns | Minutes | Failed go commands | Own tests pass | Used GoForge |
|---|---|---|---|---|---|---|---|---|---|---|---|
| products | scratch | 3 | 100% | 3/3 | $0.09 | 65,842 | 3 | 0.8 | 0.3 | 3/3 | 0/3 |
| products | scratch-production | 1 | 100% | 1/1 | $0.22 | 96,789 | 4 | 1.6 | 0.0 | 1/1 | 0/1 |
| products | goforge@v0.2.0 | 3 | 100% | 3/3 | $0.09 | 58,040 | 3 | 0.8 | 0.0 | 3/3 | 0/3 |
| products | goforge-required@v0.2.0 | 2 | 94% | 0/2 | $0.10 | 186,030 | 9 | 0.8 | 1.5 | 2/2 | 2/2 |
| products | goforge@b632824cf8a7 | 2 | 100% | 2/2 | $0.08 | 58,244 | 3 | 0.9 | 0.0 | 2/2 | 0/2 |
| products | goforge-required@b632824cf8a7 | 2 | 92% | 0/2 | $0.07 | 175,675 | 8 | 0.8 | 0.0 | 2/2 | 2/2 |
| products-sqlite | scratch | 3 | 100% | 3/3 | $0.11 | 75,796 | 4 | 2.1 | 0.0 | 3/3 | 0/3 |
| products-sqlite | goforge@v0.2.0 | 3 | 100% | 3/3 | $0.12 | 201,454 | 8 | 2.1 | 0.0 | 3/3 | 3/3 |
| products-sqlite | goforge-required@v0.2.0 | 2 | 100% | 2/2 | $0.11 | 168,049 | 8 | 2.0 | 0.5 | 2/2 | 2/2 |
| products-sqlite | goforge@b632824cf8a7 | 2 | 100% | 2/2 | $0.11 | 219,530 | 10 | 2.1 | 0.5 | 2/2 | 2/2 |
| products-sqlite | goforge-required@b632824cf8a7 | 2 | 100% | 2/2 | $0.07 | 133,419 | 6 | 1.8 | 0.5 | 2/2 | 2/2 |
| blog | scratch | 3 | 100% | 3/3 | $0.11 | 69,230 | 4 | 1.0 | 0.0 | 3/3 | 0/3 |
| blog | goforge@v0.2.0 | 3 | 100% | 3/3 | $0.11 | 62,188 | 3 | 1.0 | 0.0 | 3/3 | 0/3 |
| blog | goforge-required@v0.2.0 | 2 | 100% | 2/2 | $0.19 | 288,036 | 11 | 1.3 | 0.0 | 2/2 | 2/2 |
| blog | goforge@b632824cf8a7 | 2 | 100% | 2/2 | $0.11 | 65,933 | 3 | 0.9 | 0.5 | 2/2 | 0/2 |
| blog | goforge-required@b632824cf8a7 | 2 | 100% | 2/2 | $0.16 | 326,313 | 12 | 1.0 | 1.0 | 2/2 | 2/2 |
| orders | scratch | 3 | 100% | 3/3 | $0.13 | 92,897 | 4 | 1.5 | 1.0 | 3/3 | 0/3 |
| orders | goforge@v0.2.0 | 3 | 100% | 3/3 | $0.14 | 120,989 | 6 | 1.6 | 1.0 | 3/3 | 0/3 |
| orders | goforge-required@v0.2.0 | 2 | 100% | 2/2 | $0.23 | 204,978 | 8 | 1.9 | 0.5 | 2/2 | 2/2 |
| orders | goforge@b632824cf8a7 | 2 | 100% | 2/2 | $0.13 | 94,619 | 4 | 1.4 | 1.0 | 2/2 | 0/2 |
| orders | goforge-required@b632824cf8a7 | 2 | 100% | 2/2 | $0.20 | 173,501 | 7 | 1.6 | 0.0 | 2/2 | 2/2 |
| store | scratch | 3 | 100% | 3/3 | $0.16 | 150,393 | 6 | 1.8 | 1.0 | 3/3 | 0/3 |
| store | scratch-production | 2 | 100% | 2/2 | $0.30 | 205,313 | 7 | 3.1 | 0.0 | 2/2 | 0/2 |
| store | goforge@v0.2.0 | 2 | 100% | 2/2 | $0.17 | 164,611 | 7 | 1.8 | 0.5 | 2/2 | 0/2 |
| store | goforge-required@v0.2.0 | 2 | 99% | 1/2 | $0.24 | 535,649 | 16 | 2.1 | 0.0 | 0/2 | 2/2 |
| store | goforge@b632824cf8a7 | 2 | 100% | 2/2 | $0.12 | 128,042 | 6 | 1.4 | 0.5 | 2/2 | 1/2 |
| store | goforge-required@b632824cf8a7 | 2 | 100% | 2/2 | $0.11 | 219,416 | 10 | 1.3 | 0.5 | 2/2 | 2/2 |
| **all tasks** | scratch | 15 | 100% | 15/15 | $0.12 | 90,832 | 4 | 1.4 | 0.5 | 15/15 | 0/15 |
| **all tasks** | scratch-production | 3 | 100% | 3/3 | $0.27 | 169,138 | 6 | 2.6 | 0.0 | 3/3 | 0/3 |
| **all tasks** | goforge@v0.2.0 | 14 | 100% | 14/14 | $0.12 | 118,374 | 5 | 1.4 | 0.3 | 14/14 | 3/14 |
| **all tasks** | goforge-required@v0.2.0 | 10 | 99% | 7/10 | $0.17 | 276,548 | 10 | 1.6 | 0.5 | 8/10 | 10/10 |
| **all tasks** | goforge@b632824cf8a7 | 10 | 100% | 10/10 | $0.11 | 113,274 | 5 | 1.4 | 0.5 | 10/10 | 3/10 |
| **all tasks** | goforge-required@b632824cf8a7 | 10 | 98% | 8/10 | $0.12 | 205,664 | 9 | 1.3 | 0.4 | 10/10 | 10/10 |

## Runs

| Task | Condition | Run | Outcome | Checks | Cost | Tokens | Turns | Minutes | Failed go commands | Denied tools |
|---|---|---|---|---|---|---|---|---|---|---|
| blog | scratch | 1 | success | 23/23 | $0.11 | 61,033 | 3 | 1.0 | 0 | 0 |
| blog | scratch | 2 | success | 23/23 | $0.10 | 61,736 | 3 | 0.9 | 0 | 0 |
| blog | scratch | 3 | success | 23/23 | $0.11 | 84,921 | 5 | 1.1 | 0 | 0 |
| blog | goforge@v0.2.0 | 1 | success | 23/23 | $0.13 | 62,896 | 3 | 1.1 | 0 | 0 |
| blog | goforge@v0.2.0 | 2 | success | 23/23 | $0.11 | 63,053 | 3 | 1.0 | 0 | 0 |
| blog | goforge@v0.2.0 | 3 | success | 23/23 | $0.09 | 60,615 | 3 | 0.9 | 0 | 0 |
| blog | goforge-required@v0.2.0 | 1 | success | 23/23 | $0.21 | 309,635 | 12 | 1.3 | 0 | 0 |
| blog | goforge-required@v0.2.0 | 2 | success | 23/23 | $0.17 | 266,438 | 10 | 1.2 | 0 | 0 |
| blog | goforge@b632824cf8a7 | 1 | success | 23/23 | $0.11 | 69,161 | 3 | 1.0 | 1 | 0 |
| blog | goforge@b632824cf8a7 | 2 | success | 23/23 | $0.10 | 62,706 | 3 | 0.9 | 0 | 0 |
| blog | goforge-required@b632824cf8a7 | 1 | success | 23/23 | $0.18 | 317,059 | 12 | 1.1 | 1 | 0 |
| blog | goforge-required@b632824cf8a7 | 2 | success | 23/23 | $0.15 | 335,567 | 12 | 0.9 | 1 | 0 |
| orders | scratch | 1 | success | 20/20 | $0.14 | 110,086 | 5 | 1.7 | 1 | 0 |
| orders | scratch | 2 | success | 20/20 | $0.12 | 84,262 | 4 | 1.4 | 1 | 0 |
| orders | scratch | 3 | success | 20/20 | $0.12 | 84,345 | 4 | 1.4 | 1 | 0 |
| orders | goforge@v0.2.0 | 1 | success | 20/20 | $0.13 | 102,999 | 5 | 1.5 | 1 | 0 |
| orders | goforge@v0.2.0 | 2 | success | 20/20 | $0.13 | 127,288 | 6 | 1.6 | 1 | 0 |
| orders | goforge@v0.2.0 | 3 | success | 20/20 | $0.15 | 132,681 | 6 | 1.7 | 1 | 0 |
| orders | goforge-required@v0.2.0 | 1 | success | 20/20 | $0.21 | 178,349 | 7 | 2.0 | 1 | 0 |
| orders | goforge-required@v0.2.0 | 2 | success | 20/20 | $0.26 | 231,608 | 8 | 1.9 | 0 | 0 |
| orders | goforge@b632824cf8a7 | 1 | success | 20/20 | $0.13 | 104,192 | 5 | 1.4 | 1 | 0 |
| orders | goforge@b632824cf8a7 | 2 | success | 20/20 | $0.12 | 85,047 | 4 | 1.4 | 1 | 0 |
| orders | goforge-required@b632824cf8a7 | 1 | success | 20/20 | $0.20 | 172,400 | 7 | 1.6 | 0 | 0 |
| orders | goforge-required@b632824cf8a7 | 2 | success | 20/20 | $0.19 | 174,603 | 7 | 1.6 | 0 | 0 |
| products | scratch | 1 | success | 25/25 | $0.09 | 57,255 | 3 | 0.9 | 0 | 0 |
| products | scratch | 2 | success | 25/25 | $0.09 | 78,952 | 4 | 0.8 | 0 | 0 |
| products | scratch | 3 | success | 25/25 | $0.08 | 61,319 | 3 | 0.8 | 1 | 0 |
| products | scratch-production | 1 | success | 25/25 | $0.22 | 96,789 | 4 | 1.6 | 0 | 0 |
| products | goforge@v0.2.0 | 1 | success | 25/25 | $0.09 | 57,509 | 3 | 0.9 | 0 | 0 |
| products | goforge@v0.2.0 | 2 | success | 25/25 | $0.08 | 58,253 | 3 | 0.8 | 0 | 0 |
| products | goforge@v0.2.0 | 3 | success | 25/25 | $0.08 | 58,360 | 3 | 0.9 | 0 | 0 |
| products | goforge-required@v0.2.0 | 1 | success | 23/25 | $0.11 | 196,564 | 10 | 0.9 | 3 | 0 |
| products | goforge-required@v0.2.0 | 2 | success | 24/25 | $0.09 | 175,497 | 8 | 0.7 | 0 | 0 |
| products | goforge@b632824cf8a7 | 1 | success | 25/25 | $0.08 | 58,149 | 3 | 0.9 | 0 | 0 |
| products | goforge@b632824cf8a7 | 2 | success | 25/25 | $0.08 | 58,340 | 3 | 0.9 | 0 | 0 |
| products | goforge-required@b632824cf8a7 | 1 | success | 23/25 | $0.06 | 179,441 | 9 | 0.7 | 0 | 0 |
| products | goforge-required@b632824cf8a7 | 2 | success | 23/25 | $0.08 | 171,909 | 8 | 0.8 | 0 | 0 |
| products-sqlite | scratch | 1 | success | 27/27 | $0.11 | 75,201 | 4 | 2.2 | 0 | 0 |
| products-sqlite | scratch | 2 | success | 27/27 | $0.12 | 77,024 | 4 | 1.8 | 0 | 0 |
| products-sqlite | scratch | 3 | success | 27/27 | $0.10 | 75,165 | 4 | 2.1 | 0 | 0 |
| products-sqlite | goforge@v0.2.0 | 1 | success | 27/27 | $0.14 | 247,075 | 10 | 2.5 | 0 | 0 |
| products-sqlite | goforge@v0.2.0 | 2 | success | 27/27 | $0.11 | 168,275 | 7 | 2.0 | 0 | 0 |
| products-sqlite | goforge@v0.2.0 | 3 | success | 27/27 | $0.11 | 189,014 | 8 | 1.9 | 0 | 0 |
| products-sqlite | goforge-required@v0.2.0 | 1 | success | 27/27 | $0.10 | 165,912 | 9 | 2.0 | 1 | 0 |
| products-sqlite | goforge-required@v0.2.0 | 2 | success | 27/27 | $0.12 | 170,187 | 7 | 1.9 | 0 | 0 |
| products-sqlite | goforge@b632824cf8a7 | 1 | success | 27/27 | $0.13 | 227,913 | 10 | 2.2 | 1 | 0 |
| products-sqlite | goforge@b632824cf8a7 | 2 | success | 27/27 | $0.10 | 211,147 | 10 | 2.0 | 0 | 0 |
| products-sqlite | goforge-required@b632824cf8a7 | 1 | success | 27/27 | $0.06 | 115,308 | 6 | 1.6 | 0 | 0 |
| products-sqlite | goforge-required@b632824cf8a7 | 2 | success | 27/27 | $0.08 | 151,530 | 7 | 1.9 | 1 | 0 |
| store | scratch | 1 | success | 46/46 | $0.16 | 167,533 | 7 | 1.9 | 1 | 0 |
| store | scratch | 2 | success | 46/46 | $0.15 | 90,957 | 4 | 1.5 | 1 | 0 |
| store | scratch | 3 | success | 46/46 | $0.16 | 192,691 | 8 | 2.2 | 1 | 0 |
| store | scratch-production | 1 | success | 46/46 | $0.31 | 212,870 | 7 | 3.5 | 0 | 0 |
| store | scratch-production | 2 | success | 46/46 | $0.30 | 197,756 | 7 | 2.7 | 0 | 0 |
| store | goforge@v0.2.0 | 1 | success | 46/46 | $0.18 | 192,996 | 8 | 1.9 | 1 | 0 |
| store | goforge@v0.2.0 | 2 | success | 46/46 | $0.16 | 136,226 | 6 | 1.7 | 0 | 0 |
| store | goforge-required@v0.2.0 | 1 | success | 45/46 | $0.20 | 347,972 | 12 | 1.9 | 0 | 0 |
| store | goforge-required@v0.2.0 | 2 | success | 46/46 | $0.28 | 723,327 | 21 | 2.2 | 0 | 0 |
| store | goforge@b632824cf8a7 | 1 | success | 46/46 | $0.09 | 144,165 | 7 | 1.2 | 0 | 0 |
| store | goforge@b632824cf8a7 | 2 | success | 46/46 | $0.16 | 111,920 | 5 | 1.7 | 1 | 0 |
| store | goforge-required@b632824cf8a7 | 1 | success | 46/46 | $0.14 | 321,556 | 13 | 1.5 | 1 | 0 |
| store | goforge-required@b632824cf8a7 | 2 | success | 46/46 | $0.07 | 117,276 | 6 | 1.1 | 0 | 0 |

## Failed checks

- products · goforge-required@v0.2.0 · run 1: **list products oldest first with defaults**: item IDs = [87b4aaf35af8d75f664fbd8d89437eb6 d9379deb5e6d47180c74010ce5e836a2 5f828300120ae58a14444df7cbc804bb], want [d9379deb5e6d47180c74010ce5e836a2 87b4aaf35af8d75f664fbd8d89437eb6 5f828300120ae58a14444df7cbc804bb]
- products · goforge-required@v0.2.0 · run 1: **limit and offset page through the list**: got items [d9379deb5e6d47180c74010ce5e836a2], total 3, limit 1, offset 1; want [87b4aaf35af8d75f664fbd8d89437eb6], 3, 1, 1
- products · goforge-required@v0.2.0 · run 2: **list products oldest first with defaults**: item IDs = [8a52f37e76adfbedb4f4ff7624808d8e 97d07d7136701054a455e6b36c10791f e85278564c4a5376b0c42ac4e888b649], want [e85278564c4a5376b0c42ac4e888b649 97d07d7136701054a455e6b36c10791f 8a52f37e76adfbedb4f4ff7624808d8e]
- products · goforge-required@b632824cf8a7 · run 1: **list products oldest first with defaults**: item IDs = [5f3e131faecfda28d8bb3c3db56da1d3 7f476a0ee60a4d4e19fc3da24e6484fb 8bf0904cda85bcde765fddb60080d6a4], want [5f3e131faecfda28d8bb3c3db56da1d3 8bf0904cda85bcde765fddb60080d6a4 7f476a0ee60a4d4e19fc3da24e6484fb]
- products · goforge-required@b632824cf8a7 · run 1: **limit and offset page through the list**: got items [7f476a0ee60a4d4e19fc3da24e6484fb], total 3, limit 1, offset 1; want [8bf0904cda85bcde765fddb60080d6a4], 3, 1, 1
- products · goforge-required@b632824cf8a7 · run 2: **list products oldest first with defaults**: item IDs = [3358738e12d253c1f55cb0b075fd080a 973e5f9faeabb30b18eecccf37fc7c0b 9bc2bedd709458ca3bea778744caf2b5], want [9bc2bedd709458ca3bea778744caf2b5 3358738e12d253c1f55cb0b075fd080a 973e5f9faeabb30b18eecccf37fc7c0b]
- products · goforge-required@b632824cf8a7 · run 2: **limit and offset page through the list**: got items [973e5f9faeabb30b18eecccf37fc7c0b], total 3, limit 1, offset 1; want [3358738e12d253c1f55cb0b075fd080a], 3, 1, 1
- store · goforge-required@v0.2.0 · run 1: **deleting a category with products is a 409 and keeps it**: DELETE /categories/4c0c22cdf986867e348fb500dbf6c2ac = 500, want 409: {"error":"internal server error","request_id":"53b385b0442b13946893e9a92482aa19"}
- store · goforge-required@v0.2.0 · run 1: go test failed: ? service [no test files] --- FAIL: TestPostgresRepository (1.24s) --- FAIL: TestPostgresRepository/update (0.03s) repository_test.go:61: Get() error = category not found --- FAIL: TestPostgresRepository/delete (0.02s) repository_test.go:82: Delete() error = category not found --- FA…
- store · goforge-required@v0.2.0 · run 2: go test failed: ? service [no test files] --- FAIL: TestPostgresRepository (0.18s) --- FAIL: TestPostgresRepository/list_paginates_oldest_first (0.03s) repository_test.go:115: List(2, 0) total = 1, want 3 repository_test.go:125: List(2, 0)[0] = item-2, want item-1 repository_test.go:125: List(2, 0)[…
