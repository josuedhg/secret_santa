# 🎁 Aplicación de Intercambio (Sin contraseñas ni correos)

Una aplicación web moderna, sencilla y sin fricción para organizar tu **intercambio de regalos**, construida en Go.

**No requiere contraseñas ni correos electrónicos.** Los participantes simplemente abren su enlace de invitación único de un solo uso, registran su nombre y se inicia una sesión persistente en su navegador. Además, permite a una persona registrar a sus **hijos o familiares** desde su mismo enlace y dispositivo, con **exclusión automática garantizada para que los integrantes de una misma familia no se regalen entre sí**.

---

## ✨ Características y Reglas Garantizadas

1. **Enlaces de Invitación de Un Solo Uso**:
   - El organizador genera enlaces únicos (ej. `http://localhost:8080/join/REGALO-ABCD-1234`).
   - Cada enlace solo puede ser utilizado **una sola vez**. Al registrarse, el código queda bloqueado para esa persona.

2. **Sesión Persistente Automática (Sin Contraseñas ni Correos)**:
   - Al unirse, se guarda una cookie de sesión en el navegador (`intercambio_session`).
   - El participante puede cerrar la pestaña y volver a entrar en cualquier momento (incluso directo a `/` o `/reveal`) y la app lo reconocerá automáticamente.

3. **👨‍👩‍👧‍👦 Soporte para Familias e Hijos**:
   - Una persona con un enlace de invitación puede registrarse como titular y además agregar a sus hijos o familiares directos.
   - Ideal para niños pequeños o familiares que no cuentan con celular o correo propio.
   - Se pueden agregar familiares durante el registro inicial o posteriormente desde su panel antes del sorteo.
   - En el panel de revelación, el titular puede alternar entre pestañas para descubrir el destinatario de cada miembro familiar.

4. **Regla de Exclusión Familiar (No Regalos Intrafamiliares)**:
   - **Garantía matemática:** Ningún participante recibirá como amigo secreto a alguien de su propio núcleo familiar.
   - Se comprueba automáticamente antes del sorteo que ningún grupo supere el 50% del total de participantes (principio del palomar).

5. **Reglas Esenciales del Intercambio**:
   - **Sin autoregalos:** Nadie puede sacar su propio nombre ($Receptor \ne Emisor$).
   - **Nombres únicos:** Sin duplicados para evitar confusiones.
   - **Biyección perfecta:** Cada persona da exactamente 1 regalo y recibe exactamente 1 regalo.

6. **Revelación Interactiva con Confeti**:
   - Una vez realizado el sorteo, los participantes ven una tarjeta interactiva con animación festiva de confeti para descubrir a quién le regalan y consultar sus ideas de regalo.

7. **Panel del Organizador**:
   - Protegido por una clave de organizador (`/admin?key=...`).
   - Botón para copiar todos los enlaces disponibles en un solo clic para enviarlos por WhatsApp o Slack.
   - Visualización de grupos familiares y participantes.
   - Botón de sorteo seguro con validación de reglas.

---

## 🚀 Inicio Rápido

### Ejecutar con Go:

```bash
cd secret_santa
go run .
```

O compilar el binario ejecutable independiente:

```bash
go build -o intercambio .
./intercambio
```

Al iniciar, la consola muestra:

```text
==================================================================
🎁 APLICACIÓN DE INTERCAMBIO (Sin contraseñas ni correos)
==================================================================
 Servidor activo en:        http://localhost:8080
 Datos guardados en:        data/secretsanta.json
 Clave del Organizador:     REGALO-PASSKEY-XXXX
 Enlace directo organizador: http://localhost:8080/admin?key=REGALO-PASSKEY-XXXX
==================================================================
```

---

## ⚙️ Parámetros y Configuración

| Parámetro | Variable de Entorno | Valor por Defecto | Descripción |
|-----------|---------------------|-------------------|-------------|
| `-port` | `PORT` | `8080` | Puerto HTTP en el que escucha el servidor |
| `-base-url` | `BASE_URL` | `http://localhost:8080` | URL pública para generar los enlaces de invitación |
| `-data` | `DATA_PATH` | `data/secretsanta.json` | Ruta del archivo JSON de base de datos persistente |
| `-admin-key` | `ADMIN_KEY` | *(Auto-generada)* | Clave secreta del organizador |

---

## 🧪 Pruebas Automatizadas

Para ejecutar todas las pruebas unitarias y de integración de extremo a extremo:

```bash
go test -v ./...
```
