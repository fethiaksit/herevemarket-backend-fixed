# HereveMarket Backend

## Upload volume (Docker)

Ürün görsellerinin kalıcı olması için upload klasörü host ile container arasında mount edilmelidir:

- `/var/lib/herevemarket/uploads:/app/public/uploads`
