// Copy text to clipboard helper
function copyToClipboard(text, btnElement) {
  navigator.clipboard.writeText(text).then(() => {
    if (btnElement) {
      const originalText = btnElement.innerHTML;
      btnElement.innerHTML = "Copied! ✅";
      btnElement.disabled = true;
      setTimeout(() => {
        btnElement.innerHTML = originalText;
        btnElement.disabled = false;
      }, 2000);
    }
  }).catch(err => {
    console.error("Failed to copy:", err);
    alert("Could not copy automatically. Link: " + text);
  });
}

// Copy all unused invite links to clipboard for bulk sharing
function copyAllUnusedLinks(selector) {
  const elements = document.querySelectorAll(selector);
  const links = [];
  elements.forEach(el => {
    const link = el.getAttribute("data-link");
    if (link) links.push(link);
  });

  if (links.length === 0) {
    alert("No unused links available to copy!");
    return;
  }

  const combined = links.join("\n");
  navigator.clipboard.writeText(combined).then(() => {
    alert(`Copied ${links.length} invite links to your clipboard!`);
  }).catch(err => {
    console.error("Failed to copy:", err);
  });
}

// Interactive Gift Box Reveal
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

// Lightweight holiday confetti effect using pure canvas
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
