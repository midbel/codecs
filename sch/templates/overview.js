(() => {
    const rule = document.querySelector('#rule')
    const level = document.querySelector('#level')
    const status = document.querySelector('#status')

    function filterTableRows() {
      const rows = document.querySelectorAll('table tbody tr')
      Array.from(rows).forEach(r => {
        const levelNode = r.querySelector('td:nth-child(3)')
        const errorNode = r.querySelector('td:nth-child(6)')
        const ok1 = rule.value == "" || r.id.indexOf(rule.value) >= 0
        const ok2 = level.value == "" || levelNode.textContent == level.value
        const ok3 = status.value == "" 
          || (status.value == "succeed" && errorNode.textContent == "") 
          || (status.value == "error" && errorNode.textContent != "")
        r.style.display = ok1 && ok2 && ok3 ? "table-row" : "none"
      })
    }

    rule.addEventListener("blur", filterTableRows)
    level.addEventListener("change", filterTableRows)
    status.addEventListener("change", filterTableRows)
})();