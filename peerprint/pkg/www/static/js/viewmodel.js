
function AppViewModel(hash) {
  let self = this;
  self.serverSummary = ko.observable();
  self.storageSummary = ko.observable();
  self.instances = ko.observableArray([]);
  self.events = ko.observableArray([]);
  self.peerLogs = ko.observableArray([]);
  self.lobby = ko.observableArray([]);

  const tabel = document.querySelector('#' + hash[1]);
  if (tabel) {
    const tab = new bootstrap.Tab(tabel);
    tab.show();
  }
  self.shownTab = ko.observable(hash[1] || 'server-tab');
  self.selectedInstance = ko.observable(hash[0] !== 'null' ? hash[0] : null);
  self.setInstance = function(i) {
    self.selectedInstance(i);
  }

  self.genPSK = function() {
    throw new Error("todo");
  }

  $("#myTab a").on("shown.bs.tab", function(e) {
    self.shownTab($(e.target).attr("id"));
    self.refresh();
  });

  self._streamingGet = function(url, req, obs) {
    $.get(url, req, function(data) { 
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
    self._streamingGet("/peerLogs", {instance: self.selectedInstance()}, self.peerLogs);
  };

  self.updateTimeline = function() {
    self._streamingGet("/timeline", {instance: self.selectedInstance()}, (data) => {
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
    $.getJSON("/serverSummary", {instance: self.selectedInstance()}, function(data) { 
      self.serverSummary(data);
    });
  };
  self.updateStorageSummary = function() {
    $.getJSON("/storageSummary", {instance: self.selectedInstance()}, function(data) { 
      self.storageSummary(data);
    });
  };
  self.updateEvents = function() {
    self._streamingGet("/events", {instance: self.selectedInstance()}, self.events);
  };
  self.updateLobby = function() {
    self._streamingGet("/lobby", undefined, self.lobby);
  };
  
  
  self.updateInstances = function() {
    $.getJSON("/instances", (data) => {
      self.instances(data);
    });
  };

  self.refreshTS = ko.observable(new Date());
  self.refresh = function() {
    self.updateInstances();
    if (!self.selectedInstance()) {
      return;
    }
    switch (self.shownTab()) {
      case "timeline-tab":
        self.updateTimeline();  
        self.updatePeerLogs();
        break;
      case "server-tab":
        self.updateServerSummary();
        break;
      case "storage-tab":
        self.updateStorageSummary();
        break;
      case "events-tab":
        self.updateEvents();
        break;
      case "settings-tab":
        self.updateLobby();
        break;
    }
    self.refreshTS(new Date());
  };
  self.refresh();
  self.loop = setInterval(self.refresh, 5*1000);

  self.newConnection = function() {
    var data = $('#newconn input').toArray().reduce(function(obj, item) {
          obj[item.id] = item.value;
          return obj;
    }, {});
    $.post("/connection/new", data).done((data) => {
      console.log("Connection created");
    }).fail(() => {
      console.error("Failed to create connection");
    });
  };

  self.newAdvert = function(v) {
    var data = $('#newadvert input').toArray().reduce(function(obj, item) {
          obj[item.id] = item.value;
          return obj;
    }, {});
    $.post("/advertisement/new", data).done((data) => {
      console.log("Advert created");
    }).fail(() => {
      console.error("Failed to create advert");
    });
  };

  self.newPassword = function(v) {
    var data = $('#newpassword input').toArray().reduce(function(obj, item) {
          obj[item.id] = item.value;
          return obj;
    }, {});
    $.post("/password/new", data).done((data) => {
      console.log("password changed");
    }).fail(() => {
      console.error("Failed to change password");
    });
  };

  self.href = ko.computed(function() {
    let inst = self.selectedInstance();
    let tab = self.shownTab();
    if (!inst || !tab) {
      return;
    }
    window.location.hash =  `${inst}.${tab}`;
    console.log("set href");
    self.refresh();
  });
}

