
function AppViewModel() {
  let self = this;
  self.serverSummary = ko.observable();
  self.storageSummary = ko.observable();
  self.instances = ko.observableArray([]);
  self.selectedInstance = ko.observable(null);
  self.events = ko.observableArray([]);
  self.peerLogs = ko.observableArray([]);
  self.refreshed = ko.observable(null);

  self.refreshPeriod = ko.observable(5.0);
  console.log("Set refresh period", self.refreshPeriod()*1000);
  self.refreshPeriod.subscribe(function(v) {
    console.log("Setting refresh interval to", v);
    clearInterval(self.loop);
    self.loop = setInterval(self.refreshLoop, self.refreshPeriod()*1000);
  });

  self.selectedInstance.subscribe(function(v) {
    console.log("TODO fetch stuff for instance", v);
  }

  self._streamingGet = function(url, obs) {
    $.get(url, function(data) { 
      let result = [];
      for (let line of data.split('\n')) {
        if (line.trim() === "") {
          continue;
        }
        result.push(JSON.parse(line));
      }
      obs(result);
    });
  }

  var trace1 = {
    type: "scatter",
    mode: "lines",
    name: 'Observed peers',
    x: [],
    y: [],
    line: {color: '#17BECF'}
  }

  var layout = {
    xaxis: {
      autorange: true,
      rangeselector: {buttons: [
          {
            count: 1,
            label: '1w',
            step: 'week',
            stepmode: 'backward'
          },
          {
            count: 3,
            label: '3w',
            step: 'week',
            stepmode: 'backward'
          },
          {step: 'all'}
        ]},
      type: 'date'
    },
    yaxis: {
      autorange: true,
      type: 'linear'
    }
  };
  Plotly.newPlot('peerTimeline', [trace1], layout);

  self.updatePeerLogs = function() {
    self._streamingGet("/peerLogs", self.peerLogs);
  };

  self.updateTimeline = function() {
    self._streamingGet("/timeline", (data) => {
      trace1.x = [];
      trace1.y = [];
      for (const d of data) {
        trace1.x.push(d.Timestamp*1000);
        trace1.y.push(d.Value);
      }
      Plotly.react('peerTimeline',[trace1] , layout);
    });
  };
  self.updateServerSummary = function() {
    $.getJSON("/serverSummary", function(data) { 
      self.serverSummary(data);
    });
  };
  self.updateStorageSummary = function() {
    $.getJSON("/storageSummary", function(data) { 
      self.storageSummary(data);
    });
  };
  self.updateEvents = function() {
    self._streamingGet("/events", self.events);
  };
  
  self.updateInstances = function() {
    $.getJSON("/instances", (data) => {
      self.instances(data);
    });
  };

  self.refreshTS = ko.observable(new Date());
  self.refreshLoop = function() {
    console.log("loop");
    self.updateInstances();
    self.updateTimeline();
    self.updatePeerLogs();
    self.updateServerSummary();
    self.updateStorageSummary();
    self.updateEvents();
    self.refreshTS(new Date());
  };
  self.refreshLoop();
  self.loop = setInterval(self.refreshLoop, self.refreshPeriod()*1000);
  console.log(self.loop);

}

