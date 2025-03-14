(() => {

  function createLinkFromItem(item, height) {
    const el = document.createElement("a")

    el.style.display = "block"
    el.style.height = height + "px"
    el.style.border = "1px solid #E0E0E0"
    if (item.classList.contains("failure")) {
      el.classList.add("failure")
    }
    el.href = "#" + item.id

    return el
  }

  function createSidebar() {
    const sidebar = document.createElement("div")

    sidebar.style.position = "fixed"
    sidebar.style.zIndex = "9999"
    sidebar.style.top = 0
    sidebar.style.bottom = 0
    sidebar.style.right = 0
    sidebar.style.backgroundColor = "rgba(255,255,255,0.8)"
    sidebar.style.width = "160px"
    sidebar.style.borderLeft = "1px solid #E0E0E0"

    return sidebar    
  }

  document.addEventListener("DOMContentLoaded", () => {
    const all = document.querySelectorAll("pre")
    if (all.length == 0) {
      return
    }
    const total = all.length

    sidebar = createSidebar()
    document.querySelector("body").append(sidebar)

    const sideHeight = sidebar.offsetHeight
    const itemHeight = sideHeight / total

    Array.from(all).forEach((item) => {
      const el = createLinkFromItem(item, itemHeight)
      sidebar.append(el)
    })
  })
})();