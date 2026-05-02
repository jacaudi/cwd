//Javascript to gather SWPC, Tsunami, Earthquake, and Volcano data for CWD page
//May 2021 - Created
//February 2023 - SWPC forecast restored
//April 2023 - Tsunami, Earthquake, and Volcano alerts restored
//April 2024 - Lowered threshold of Volcano alerts (now will display at a "Watch" level). Also modified title text of Tsunamis, Earthquakes, and Volcanoes. 
//May 2024 - Add count of Earthquake and Volcano alerts

//set global variables to use in "get_alerts_test.js"
var neqw = 0;
var nvlw = 0;
var leqw = '';
var lvlw = '';

//set variables and arrays to gather data
var storedText; 
var storedTextb;
var storedTextc;
var storedTextd;

let fdate = new Array();
let s1 = new Array();
let r1 = new Array();
let r3 = new Array();
let g = new Array();
let gt = new Array();
let gn = new Array();


function getdata() {
//pull in SWPC forecast data	
fetch(urla)
  .then(function(response) {
    response.json().then(function(json) {
      storedText = json;
      getforecast();
    });
  });
}

function getdatab() {
//pull in USGS earthquake data

fetch(urlb)
  .then(function(response) {
    response.json().then(function(json) {
      storedTextb = json;
      geteq();
    });
  });
}


function getdatac() {
//pull in NWS API data	
fetch(urlc)
  .then(function(response) {
    response.json().then(function(json) {
      storedTextc = json;
      getts();
    });
  });
}

function getdatad() {
fetch(urld)
  .then(response => response.text())
  .then(data => {
    const parser = new DOMParser();
    const storedTextd = parser.parseFromString(data, "application/xml");
    temp = storedTextd.getElementsByTagName("item");
    vol = '';
    volc = 'no';
    nvlw = 0;	  
    for (i=0; i < temp.length; i++) {
      voltitle = temp[i].getElementsByTagName("volcano:alertlevel")[0].childNodes[0].nodeValue;
      //Set threshold of what level alert to pull	    
      if ((voltitle == "WATCH") || (voltitle == "WARNING")){
        volc = 'yes';
	if (voltitle == "WARNING") {      
          nvlw++;
	}	
        //pull out needed Volcano info
        link = temp[i].getElementsByTagName("link")[0].childNodes[0].nodeValue;
        title = temp[i].getElementsByTagName("title")[0].childNodes[0].nodeValue;
        description = temp[i].getElementsByTagName("description")[0].childNodes[0].nodeValue;
        alertlevel = temp[i].getElementsByTagName("volcano:alertlevel")[0].childNodes[0].nodeValue;
        colorlevel = temp[i].getElementsByTagName("volcano:colorcode")[0].childNodes[0].nodeValue;
        if (colorlevel == "RED") {
          colorlevel = "<span class='vol-background-red'>RED</span>";
        } else if (colorlevel == "ORANGE") {
          colorlevel = "<span class='vol-background-orange'>ORANGE</span>";
        }
        titlebreak = title.split(" - ");
        subtitlebreak = titlebreak[0].split(" ");
        level = subtitlebreak[subtitlebreak.length-1];
        subtitlebreak.pop();
        shorttitle = subtitlebreak.join(" ");
        shortdescription = description.split(" - ");


	wfolink = '<a href="https://www.usgs.gov/programs/VHP/volcano-updates" target="_blank">USGS</a>';
        lvlw = lvlw + '<div class="row-opt-e"><div class="col-opt-za">' + 'Volcano with Watch/Warning Alert Level' + '</div><div class="col-opt-zb">' + wfolink + '</div><div class="col-opt-zc">' + 'NA' + '</div><div class="col-opt-zd">NA</div><div class="col-opt-ze">' + 'NA' + '</div></div>' ;
      
        vol = vol + '<div class="div-opt-vol"><a href="' + link + '" target="_blank">' + shorttitle + '</a> - ' + colorlevel + '/' + alertlevel + '<abr><br>'+ shortdescription[0] + ' UTC - ' + titlebreak[1] + '</div>';

      }
   }
   if (volc == 'yes') {
     document.getElementById('vol').innerHTML = "Volcanoes <span style='font-weight: normal'> (<i><a href='https://www.usgs.gov/programs/VHP/volcano-updates' target='_blank'>USGS</a></i> watch/warning alerts)</span>";
     document.getElementById('vl').innerHTML = vol;
   } else {
     document.getElementById('vl').innerHTML = "<br>No active USGS watch/warning level alerts. Visit <i><a href='https://www.usgs.gov/programs/VHP/volcano-updates' target='_blank'>USGS</a></i> for more info.";
   }

  })
  .catch(console.error);
}

function getforecast() {
//collect forecast data from SWPC
  let temp = new Array();
//  console.log(storedText);
  for (i=0; i<3 ; i++) {
    fdate[i] = storedText[i+1].DateStamp;
    gn[i] = storedText[i+1].G.Scale; 
    gt[i] = storedText[i+1].G.Text;
    r1[i] = storedText[i+1].R.MinorProb;
    r3[i] = storedText[i+1].R.MajorProb;
    s1[i] = storedText[i+1].S.Prob;
  }
 
  //test to make sure actually grabbed dates
    //gathered data, now qc it
    qcdata();
//  } else { 
//    donotdisplay();
//  }

}

function qcdata() {
//convert forecast to integers
    r1    = r1.map(convertnum)
    r3    = r3.map(convertnum)
    s1    = s1.map(convertnum)

    function convertnum(num) {
      return parseInt(num);
    }

   //Only display SWPC data if valid numbers
   if (r1.includes(NaN) == false) {
     display();
   } else {
     donotdisplay();
   }

}

function donotdisplay() {
  document.getElementById("SWPCy").className = "hide";
  document.getElementById("SWPCn").className = "row-opt-a";
}

function display() {
//Display SWPC data
  for (i=0; i < fdate.length; i++){
    document.getElementById("swpc-fd-"+[i+1]).textContent = "Predicted for " + fdate[i]; 
    document.getElementById("swpc-r1-"+[i+1]).textContent = r1[i] + "%";
    document.getElementById("swpc-r3-"+[i+1]).textContent = r3[i] + "%";
    document.getElementById("swpc-s1-"+[i+1]).textContent = s1[i] + "%"; 
    document.getElementById("swpc-gt-"+[i+1]).textContent = gt[i];
    if (gn[i] == 0) {
      document.getElementById("swpc-gtc-"+[i+1]).textContent = "G";
    } else {
      document.getElementById("swpc-gtc-"+[i+1]).textContent = "G" + gn[i];
    }
    document.getElementById("swpc-g-"+[i+1]).className = "col-opt-" + gt[i];
    document.getElementById("swpc-gtc-"+[i+1]).className = "div-opt-" + gt[i];
  } 
  document.getElementById("SWPCn").className = "hide";
  document.getElementById("SWPCy").className = "row-opt-a";
}

function geteq() {
  //gather and display Earthquake Data
  var list = '';
  var time;
  var lat;
  var lon;
  var dep;
  var line1;
  var line2;
  neqw = 0;	
  if (storedTextb.features.length > 0) { 
    //loop through and gather data from USGS json file
    for (i=0; i < storedTextb.features.length; i++){
      neqw++;	    
      line1 = "<a href='"+ storedTextb.features[i].properties.url +"' target='_blank'>" + storedTextb.features[i].properties.title + '</a>';
      time = storedTextb.features[i].properties.time;
      d = new Date(time);
      y = d.getUTCFullYear();
      m = d.getUTCMonth()+1;
      m = ("0" + m).slice(-2);
      dy = d.getUTCDate();
      dy = ("0" + dy).slice(-2);
      h = d.getUTCHours();
      h = ("0" + h).slice(-2);
      mn = d.getUTCMinutes();
      mn = ("0" + mn).slice(-2);
      s = d.getUTCSeconds();
      s = ("0" + s).slice(-2);
      vdate = y + '-' + m + '-' + dy + ' ' + h + ':' + mn + ':' + s + '(UTC)';
      latD = 'N';
      lonD = 'E';
      lat = storedTextb.features[i].geometry.coordinates[1];
      lon = storedTextb.features[i].geometry.coordinates[0];
      if (lat < 0) latD = 'S';
      if (lon < 0) lonD = 'W';
      dep = storedTextb.features[i].geometry.coordinates[2];  
//      line2 = vdate + ' | ' + lat + '&#176' + latD + ' ' + lon + '&#176' + lonD + ' ' + ' | ' + dep + ' meters';
//      list = list + "<div class='div-opt-eq'>" + line1 + "<br>" + line2 +  "</div>";
            line2 = vdate + ' | ' + lat + '&#176' + latD + ' ' + lon + '&#176' + lonD + ' ' + ' | ' + dep + ' meters';
      line1 = "<a href='"+ storedTextb.features[i].properties.url +"' target='_blank'>" + storedTextb.features[i].properties.title + "</a>";
      line3 = vdate;
      line2 = lat + '&#176' + latD + ' ' + lon + '&#176' + lonD + ' ' + ' | ' + dep + ' meters';
//      list = list + "<div class='div-opt-eq'>" + line1 + "<br></div>";
      list = list + "<div class='div-opt-eq'>" + line1 + "<br>" + line2 + "<br>" + line3 + "</div>";
      wfolink = '<a href="https://earthquake.usgs.gov/earthquakes/map/" target="_blank">USGS</a>';
      leqw = leqw + '<div class="row-opt-e"><div class="col-opt-za">' + 'Significant Earthquake' + '</div><div class="col-opt-zb">' + wfolink + '</div><div class="col-opt-zc">' + 'NA' + '</div><div class="col-opt-zd">NA</div><div class="col-opt-ze">' + 'NA' + '</div></div>' ;
    }
    document.getElementById('eqk').innerHTML = "Earthquakes <span style='font-weight: normal'>(<a href='https://earthquake.usgs.gov/earthquakes/browse/significant.php#sigdef' title='What makes an earthquake significant?' target='_blank'>USGS significant events</a> in the past 24 hrs) </span>";	  
    document.getElementById('eq').innerHTML = list;
  } else {
    //no features to display
    list = "<br>No <i><a href='https://earthquake.usgs.gov/earthquakes/browse/significant.php#sigdef' title='What makes an earthquake significant?' target='_blank'>significant earthquakes</a></i> in the past 24 hrs. Visit <i><a href='https://earthquake.usgs.gov/earthquakes/map/?extent=10.66061,-148.44727&extent=58.53959,-41.57227' target='_blank'>USGS</a></i> for more info.";
   document.getElementById('eq').innerHTML = list;	  
  }
}

function getts(){
   var tsu = '';
   var tsuc = 'no';
   //loop through active tsunami alerts
   
   for (i=0; i<storedTextc.features.length; i++){
     var temp;
     var link;
     var site;
     var prod;

     temp = String(storedTextc.features[i].properties.parameters.AWIPSidentifier);
     prod = temp.slice(0,3);
//     if (prod == "RFW") {
     if (prod == "TSU") {	   
       tsuc = 'yes';
       link = 'https://forecast.weather.gov/product.php?site=NWS&product=' + prod +'&issuedby=' + temp.slice(3,6) ;
       tsu = tsu + '<div class="div-opt-ts"><a href="' + link + '" target="_blank">' + storedTextc.features[i].properties.headline + '</a></div>';
     }

//****************************************************
   }

   if (tsuc == 'yes') {
     document.getElementById('tst').innerHTML = "Tsunami <span style='font-weight: normal'> (<i><a href='https://tsunami.gov' target='_blank'>NWS</a></i> watches/warnings/advisories) </span>";
     document.getElementById('ts').innerHTML = tsu;
   } else {
     document.getElementById('ts').innerHTML = "<br>No active tsunami watches/warnings. Visit <i><a href='https://www.tsunami.gov' target='_blank'>tsunami.gov</a></i> for more info.";
   }     
}


