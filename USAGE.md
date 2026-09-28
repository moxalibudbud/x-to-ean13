## Usage

| Flag       | Default                  | Description                                                  |
|------------|--------------------------|----------------------------------------------------------------|
| `-header`  | `false`                  | Set if the input CSV has a header row to skip                |
| `-prefix`  | `200`                    | Company prefix used when generating the EAN-13               |
| `-output`  | `<input>.ean13.csv`      | Output CSV file path                                          |
| `-ledger`  | `<input>.ledger.csv`     | Ledger file recording word→EAN-13 allocations                 |

The first column of each input row is treated as the word. The output CSV has
two columns: the word and its generated `ean13`. If `-header` is set, the
output header reuses the source's first-column name and appends `ean13`.

Example:

```sh
./x-to-ean13 -header -prefix 200 words.csv
# ean13 file generated. N row(s) to words.ean13.csv (N newly allocated, 0 reused from words.ledger.csv)