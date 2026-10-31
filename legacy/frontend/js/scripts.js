document.addEventListener("DOMContentLoaded", () => {
  const myList = document.getElementById("quote-list");
  const myRequest = new Request("http://localhost:8080/backend/api/api.json");

  fetch(myRequest)
    .then((response) => response.json())
    .then((data) => {
      console.log(data);
      for (const lista of data) {
        myList.innerHTML += `
        <tr>
        <th>
        #
        </th>
        <th> 
            ${lista.Equipe} 
        </th>
        <th> 
            ${lista.PTS} 
        </th>
        <th> 
            ${lista.J} 
        </th>
        <th> 
            ${lista.V} 
        </th>
        <th> 
            ${lista.E} 
        </th>
        <th> 
            ${lista.D} 
        </th>
        <th> 
            ${lista.GP} 
        </th>
        <th> 
            ${lista.GC} 
        </th>
        <th> 
            ${lista.SG} 
        </th>
        <th> 
            ${lista.CA} 
        </th>
        <th> 
            ${lista.CV} 
        </th>
        <th> 
            ${lista.POR} 
         </th>
        </tr>`;
      }
    })

    .catch(console.error);
});
