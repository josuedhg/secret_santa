# 🎁 Aplicación de Intercambio (Sin contraseñas ni correos)

Una aplicación web moderna, sencilla y sin fricción para organizar tu **intercambio de regalos**, construida en Go.

**No requiere contraseñas ni correos electrónicos.** Los participantes simplemente abren su enlace de invitación único de un solo uso, registran su nombre y se inicia una sesión persistente en su navegador. Cuando el organizador realiza el sorteo, cada participante entra a la página y descubre inmediatamente a quién le toca regalar.

---

## ✨ Características y Reglas Garantizadas

1. **Enlaces de Invitación de Un Solo Uso**:
   - El organizador genera enlaces únicos (ej. `http://localhost:8080/join/REGALO-ABCD-1234`).
   - Cada enlace solo puede ser utilizado **una sola vez**. Al registrarse, el código queda bloqueado para esa persona.

2. **Sesión Persistente Automática (Sin Contraseñas)**:
   - Al unirse, se guarda una cookie de sesión en el navegador (`intercambio_session`).
   - El participante puede cerrar la pestaña y volver a entrar en cualquier momento (incluso directo a `/` o `/reveal`) y la app lo reconocerá automáticamente.

3. **Regla 1: Nadie Puede Sacar su Propio Nombre**:
   - El algoritmo matemático (ciclo hamiltoniano aleatorizado con `crypto/rand`) garantiza que para cualquier participante $i$, $\text{Receptor} \ne \text{Emisor}$.

4. **Regla 2: Nombres No Duplicados**:
   - La aplicación valida que los nombres no se repitan (insensible a mayúsculas/minúsculas) para evitar confusiones.

5. **Regla 3: Todos Dan y Todos Reciben Exactamente Un Regalo**:
   - Todos los participantes tienen exactamente un destinatario y son el destinatario de exactamente un participante ($1$ a $1$).
   - Al formar un ciclo completo ($P_0 \to P_1 \to \dots \to P_{n-1} \to P_0$), se evitan intercambios aislados de solo 2 personas cuando el grupo es de 3 o más.

6. **Revelación Interactiva con Confeti**:
   - Una vez realizado el sorteo, los participantes ven una tarjeta interactiva con animación de confeti para descubrir a quién le regalan y consultar sus ideas de regalo.

7. **Panel del Organizador**:
   - Protegido por una clave de organizador (se guarda en sesión con un clic vía `/admin?key=...`).
   - Botón para copiar todos los enlaces disponibles en un solo clic para enviarlos por WhatsApp o Slack.
   - Seguimiento en tiempo real de los participantes registrados.
   - Botón de sorteo seguro con confirmación.

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
