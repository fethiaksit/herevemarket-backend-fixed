# Endpoint Özeti

## Auth (User)
- `POST /auth/register` → Yeni kullanıcı kaydı (email, password, name). Başarılıysa access token döner.
- `POST /auth/login` → Kullanıcı girişi (email, password). Başarılıysa access token döner.
- `GET /auth/me` → Giriş yapan kullanıcı bilgileri + adresler.

## Adres Yönetimi (User, giriş gerekli)
- `GET /user/addresses`
- `POST /user/addresses`
- `PUT /user/addresses/:id`
- `DELETE /user/addresses/:id`

## Sipariş (Guest/User)
- `POST /orders` → Token varsa userId ile, yoksa guest olarak kayıt.
