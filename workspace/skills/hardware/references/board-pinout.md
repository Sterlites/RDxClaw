# Board Pinout & Pinmux Reference

This document provides examples of how to configure I2C and SPI buses on various Linux-based boards.

## Example Board (Generic ARM/RISC-V)

### I2C Buses

| Bus | Pins | Notes |
|-----|------|-------|
| I2C-1 | Depends on board | Common for sensors |
| I2C-Software | Configurable | Slower but no pin conflicts |

### SPI Buses

| Bus | Pins | Notes |
|-----|------|-------|
| SPI-1 | SCK, MOSI, MISO, CS | High speed interface |

### Common Setup Steps

```bash
# 1. Load i2c-dev module
modprobe i2c-dev

# 2. Configure pinmux if required (example using devmem)
# This is board specific
# devmem 0x030010D0 b 0x1

# 3. Verify
ls /dev/i2c-*
ls /dev/spidev*
```

---

## Common Issues

### devmem not found
The `devmem` utility may not be in the default image. Options:
- Use `busybox devmem` if busybox is installed
- Download from your board manufacturer's repository

### Dynamic bus numbering
I2C adapter numbers can change between boots depending on driver load order. Always use `i2c detect` to find current bus assignments rather than hardcoding bus numbers.

### Permissions
`/dev/i2c-*` and `/dev/spidev*` typically require root access. Options:
- Run the agent as root
- Add user to `i2c` and `spi` groups
- Create udev rules: `SUBSYSTEM=="i2c-dev", MODE="0666"`
