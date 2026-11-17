// Helper para copiar texto al portapapeles
function copyToClipboard(text, btnElement) {
  navigator.clipboard.writeText(text).then(() => {
    if (btnElement) {
      const originalText = btnElement.innerHTML;
      btnElement.innerHTML = "¡Copiado! ✅";
      btnElement.disabled = true;
      setTimeout(() => {
        btnElement.innerHTML = originalText;
        btnElement.disabled = false;
      }, 2000);
    }
  }).catch(err => {
    console.error("Error al copiar:", err);
    alert("No se pudo copiar automáticamente. Enlace: " + text);
  });
}

// Copiar todos los enlaces disponibles al portapapeles para compartirlos
function copyAllUnusedLinks(selector) {
  const elements = document.querySelectorAll(selector);
  const links = [];
  elements.forEach(el => {
    const link = el.getAttribute("data-link");
    if (link) links.push(link);
  });

  if (links.length === 0) {
    alert("¡No hay enlaces disponibles para copiar!");
    return;
  }

  const combined = links.join("\n");
  navigator.clipboard.writeText(combined).then(() => {
    alert(`¡Se copiaron ${links.length} enlaces de invitación al portapapeles!`);
  }).catch(err => {
    console.error("Error al copiar:", err);
  });
}

// Experiencia interactiva para descubrir a quién le regalas
function unwrapGift() {
  const boxBtn = document.getElementById("giftBoxBtn");
  const revealResult = document.getElementById("revealResult");

  if (boxBtn && revealResult) {
    boxBtn.style.transform = "scale(0.8) rotate(15deg)";
    boxBtn.style.opacity = "0";
    setTimeout(() => {
      boxBtn.style.display = "none";
      revealResult.style.display = "block";
      triggerHolidayConfetti();
    }, 250);
  }
}

// Animación festiva de confeti con canvas nativo
function triggerHolidayConfetti() {
  const canvas = document.createElement("canvas");
  canvas.id = "confetti-canvas";
  canvas.style.position = "fixed";
  canvas.style.top = "0";
  canvas.style.left = "0";
  canvas.style.width = "100%";
  canvas.style.height = "100%";
  canvas.style.pointerEvents = "none";
  canvas.style.zIndex = "999999";
  document.body.appendChild(canvas);

  const ctx = canvas.getContext("2d");
  canvas.width = window.innerWidth;
  canvas.height = window.innerHeight;

  const colors = ["#dc2626", "#15803d", "#f59e0b", "#3b82f6", "#ffffff"];
  const pieces = [];
  for (let i = 0; i < 80; i++) {
    pieces.push({
      x: Math.random() * canvas.width,
      y: Math.random() * -canvas.height,
      size: Math.random() * 8 + 4,
      color: colors[Math.floor(Math.random() * colors.length)],
      speedY: Math.random() * 3 + 2,
      speedX: Math.random() * 2 - 1,
      rotation: Math.random() * 360,
      rotSpeed: Math.random() * 4 - 2
    });
  }

  let frames = 0;
  function render() {
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    pieces.forEach(p => {
      p.y += p.speedY;
      p.x += p.speedX;
      p.rotation += p.rotSpeed;

      ctx.save();
      ctx.translate(p.x, p.y);
      ctx.rotate((p.rotation * Math.PI) / 180);
      ctx.fillStyle = p.color;
      ctx.fillRect(-p.size / 2, -p.size / 2, p.size, p.size);
      ctx.restore();
    });

    frames++;
    if (frames < 240) {
      requestAnimationFrame(render);
    } else {
      canvas.remove();
    }
  }
  requestAnimationFrame(render);
}
