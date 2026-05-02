//Javascript to gather active alerts to display on CWD webpage
//Created May 2023

//3/23/2025 - Changed pull of "Excessive Heat Warning" to "Extreme Heat Warning" to account for HazSimp SCN 24-88
//2/19/2026 - Added SWPC alerts for observed S2 and S3 conditions (P112A and P113A).

//*****Gather alert data from API (Tsunami, Tornado, Severe TStorm, Flash Flood, Tropical, High Winds, Fire Weather, WinterWx, Extreme Heat, Extreme Cold)*****
//***************************************************************************
//
var storedAlertTexta; 
var storedAlertTexte;
var nswp = 0;

function getAlertsApi() {
//pull in NWS API data
fetch(urlc)
  .then(function(response) {
    response.json().then(function(json) {
      storedAlertTexta = json;
      getapialerts();
    });
  });
}

function getAlertsSWPC() {
//pull in SWPC data
fetch(urle)
  .then(function(response) {
    response.json().then(function(json) {
      storedAlertTexte = json;
      getswpcalerts();
    });
  });
}

function getswpcalerts() {
//  alertsToDisplay = alertsToDisplay + "and SWPC";
//  document.getElementById('DisplayAlerts').innerHTML = alertsToDisplay;
  nswp = 0;
  //date the most recent product was issued	
  idate = String(storedAlertTexte[0].issue_datetime);
  
  //Website will display SWPC alerts issued within the last "X" hours?
  x = 24;
  //Determine how far back to look for SWPC alerts for based on current time
  const d = new Date();
  d.setHours(d.getHours() - x);
  y = d.getUTCFullYear();
  m = d.getUTCMonth()+1;
  m = ("0" + m).slice(-2);
  dy = d.getUTCDate();
  dy = ("0" + dy).slice(-2);
  h = d.getUTCHours();
  h = ("0" + h).slice(-2);
  mn = d.getUTCMinutes();
  mn = ("0" + mn).slice(-2);
  cdate = y +  m + dy + h + mn;

  for (i=0; i<storedAlertTexte.length; i++){
    tdate = String(storedAlertTexte[i].issue_datetime);
    mdate = tdate.replace(/[-,:,., ]/g,'');
    idate = mdate.substr(0,12);

    //check if issue date was within the last X hours
    if (idate > cdate) {
       //check what the product is. Look for the following products...
         //K08A = Geomagnetic K-index of 8 has been reached (G4)
	 //K09A = Geomagnetic K-index of 9 has been reached (G5)
	 //P12A = Moderate solar radiation storm has been observed (S2)
	 //P13A = Strong solar radiation storm has been observed (S3)
       
       temp = String(storedAlertTexte[i].product_id);
       if ((temp == "K08A") || (temp == "K09A") || (temp == "P12A") || (temp == "P13A")) {
         nswp++;
         //break;
       }
    } else {
      break;
    }
  }
}

function getapialerts() {
  //Collect API data
  var ntsu = 0;
  var ntor = 0;
  var nsvr = 0;
  var nffw = 0;	
  var nhww = 0;
  var rdfw = 0;
  var wwbw = 0;
  var exhw = 0;
  var excw = 0;
  var trpw = 0;
  let alertsToDisplay = "";
  for (i=0; i<storedAlertTexta.features.length; i++){
    var temp;
    var tsulink;
    var prod;

    temp = String(storedAlertTexta.features[i].properties.parameters.AWIPSidentifier);
    prod = temp.slice(0,3);
    apievent = String(storedAlertTexta.features[i].properties.event);
    //Collect Tsunami Alerts	   
    if (prod == "TSU") {
      ntsu++;
      tsulink = 'https://forecast.weather.gov/product.php?site=NWS&product=' + prod +'&issuedby=' + temp.slice(3,6) ;
    }
    
    //Collect Tornado Alerts
    if (apievent == "Tornado Warning") {
      ntor++;
    }

    //Collect Severe Weather Alerts
    if (apievent == "Severe Thunderstorm Warning") {
      nsvr++;
    }

    //Collect Flash Flood Alerts
    if (apievent == "Flash Flood Warning") {
      nffw++;
    }
    //Collect Tropical Alerts
    if ((apievent == "Storm Surge Warning") || (apievent == "Hurricane Warning") || (apievent == "Typhoon Warning") || (apievent == "Tropical Storm Warning")){
      trpw++;
    }	  
    //Collect High Wind Alerts
    if ((apievent == "High Wind Warning") || (apievent == "Extreme Wind Warning")) {
      nhww++;
    }
     //Collect Fire Weather Alerts
    if (apievent == "Red Flag Warning") {
      rdfw++;
    }
    //Collect Winter Weather alerts
     if ((apievent == "Winter Storm Warning") || (apievent == "Blizzard Warning") || (apievent == "Ice Storm Warning") || (apievent == "Snow Squall Warning")) {
      wwbw++;
    }
     //Collect Extreme Heat Alerts
    if (apievent == "Extreme Heat Warning") {
      exhw++;
    }
    //Collect Extreme Cold Alerts
    if (apievent == "Extreme Cold Warning") {
      excw++;
    }
  }
    
  //Build string of active alerts to display on CWD page
   if (ntsu > 0) {
     alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://www.nco.ncep.noaa.gov/status/cwd/#midpage" title="Number of active Tsunami Watches, Warnings, and Advisories"><img class="alert-img" src="/status/css/images/icon-tsunami.png" alt="alert icon" width="24" height"24">' + ntsu + '</a>';	  
   }
   if (neqw > 0) {
     alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://www.nco.ncep.noaa.gov/status/cwd/#midpage" title="Number of USGS Significant Earthquakes in the past 24 hours"><img class="alert-img" src="/status/css/images/icon-earthquake.png" alt="alert icon" width="24" height"24">' + neqw + '</a>';
   }
   if (nvlw > 0) {
     alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://www.nco.ncep.noaa.gov/status/cwd/#midpage" title="Number of active warning level volcano alerts"><img class="alert-img" src="/status/css/images/icon-volcano.png" alt="alert icon" width="24" height"24">' + nvlw + '</a>';
   }
   if (ntor > 0) {
     alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://forecast.weather.gov/wwamap/wwatxtget.php?cwa=usa&wwa=Tornado%20Warning" title="Number of active Tornado Warnings"><img class="alert-img" src="/status/css/images/icon-tornado.png" alt="alert icon" width="24" height"24">' + ntor + '</a>';
   }
   if (nsvr > 0) {
    alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://forecast.weather.gov/wwamap/wwatxtget.php?cwa=usa&wwa=Severe%20Thunderstorm%20Warning" title="Number of active Severe Thunderstorm Warnings"><img class="alert-img" src="/status/css/images/icon-lightning.png" alt="alert icon" width="24" height"24">' + nsvr + '</a>';
   }
   if (nffw > 0) {
     alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://forecast.weather.gov/wwamap/wwatxtget.php?cwa=usa&wwa=Flash%20Flood%20Warning" title="Number of active Flash Flood Warnings"><img class="alert-img" src="/status/css/images/icon-flood.png" alt="alert icon" width="24" height"24">' + nffw + '</a>';
   }
   if (trpw > 0) {
     alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://www.nhc.noaa.gov/" title="Number of active Storm Surge, Hurricane, Typhoon, and Tropical Storm Warnings"><img class="alert-img" src="/status/css/images/icon-tropical.png" alt="alert icon" width="24" height"24">' + trpw + '</a>';
   }
   if (nhww > 0) {
     alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://forecast.weather.gov/wwamap/wwatxtget.php?cwa=usa&wwa=High%20Wind%20Warning" title="Number of active Extreme Wind and High Wind Warnings"><img class="alert-img" src="/status/css/images/icon-wind.png" alt="alert icon" width="24" height"24">' + nhww + '</a>';
   }
   if (rdfw > 0) {
     alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://forecast.weather.gov/wwamap/wwatxtget.php?cwa=usa&wwa=Red Flag Warning" title="Number of active Red Flag Warnings"><img class="alert-img" src="/status/css/images/icon-wildfire.png" alt="alert icon" width="24" height"24">' + rdfw + '</a>';
   }
   if (wwbw > 0) {
      alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://www.weather.gov" title="Number of active Blizzard, Ice Storm, Winter Storm, and Snow Squall Warnings"><img class="alert-img" src="/status/css/images/icon-winter.png" alt="alert icon" width="24" height"24">' + wwbw + '</a>';
   }
   if (exhw > 0) {
     alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://forecast.weather.gov/wwamap/wwatxtget.php?cwa=usa&wwa=Extreme%20Heat%20Warning" title="Number of active Extreme Heat Warnings"><img class="alert-img" src="/status/css/images/icon-heat.png" alt="alert icon" width="24" height"24">' + exhw + '</a>';
   }
   if (excw > 0) {
     alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://forecast.weather.gov/wwamap/wwatxtget.php?cwa=usa&wwa=Extreme%20Cold%20Warning" title="Number of active Extreme Cold Warnings"><img class="alert-img" src="/status/css/images/icon-cold.png" alt="alert icon" width="24" height"24">' + excw + '</a>';
   }
   if (nswp > 0) {
     alertsToDisplay = alertsToDisplay + '<a target="_blank" href="https://www.swpc.noaa.gov/products/alerts-watches-and-warnings" title="A Geomagnetic Storm G4 or greater and/or a Solar Radiation Storm of S2 or greater has been observed within the last 24 hours"><img class="alert-img" src="/status/css/images/icon-space.png" alt="alert icon" width="24" height"24">' + nswp + '</a>';
   }
	
  if (alertsToDisplay === "") {
    alertsToDisplay = '<span class="no-alerts">No active alerts.</span>';	  
  }	  
  document.getElementById('DisplayAlerts').innerHTML = alertsToDisplay;	
//  document.getElementById('DisplayAlertsTop').innerHTML = alertsToDisplay;
}

function getTime () {
     var dt= new Date();
     var hours = String(dt.getUTCHours()).padStart(2, '0');	
     var minutes = String(dt.getUTCMinutes()).padStart(2, '0');	
     document.getElementById("DisplayTime").textContent = "Alerts as of " + hours + minutes + "Z";
}





