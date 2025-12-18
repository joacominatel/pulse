## Usage

### Specific community
```bash
python scripts/noise_generator.py --community-id <uuid> --token <jwt>
```

### Random across all communities
```bash
python scripts/noise_generator.py --random --token <jwt>
```

### Options
```bash
python scripts/noise_generator.py \
  --community-id <uuid> \
  --token <jwt> \
  --count 100 \
  --delay 50 \
  --verbose
```

## Event Distribution

The script simulates realistic event patterns:
- View: 40% (most common, low weight)
- Reaction: 25% 
- Comment: 20%
- Post: 10% (high weight)
- Join: 5% (high weight)
- Share: 5%

## Examples

### quick test
```bash
python scripts/noise_generator.py -c abc123 -t eyJ... -n 10 -v
```

### load test
```bash
python scripts/noise_generator.py -r -t eyJ... -n 500 -d 10
```
