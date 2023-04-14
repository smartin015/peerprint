

function PeerTimeline(id, title, xlabel, ylabel) {
  var self = this;

  let trace1 = {
    type: "scatter",
    mode: "lines",
    name: 'Observed peers',
    x: [],
    y: [],
    line: {color: '#17BECF'}
  }

  var layout = {
    title: title,
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
      type: 'date',
      title: xlabel,
    },
    yaxis: {
      autorange: true,
      type: 'linear',
      title: ylabel,
    }
  };
  Plotly.newPlot(id, [trace1], layout);

  self.update = function(data) {
      trace1.x = [];
      trace1.y = [];
      for (const d of data) {
        trace1.x.push(d.Timestamp*1000);
        trace1.y.push(d.Value);
      }
      Plotly.react(id ,[trace1] , layout);
  };
}

function PeerGeoMap(id, title) {
	var self = this;

	let data = {
			type: 'scattergeo',
			mode: 'markers',
			lon: [],
			lat: [],
			marker: {
          color: '#4444ff',
					size: 10,
					line: {
							width: 1
					}
			},
	};

	var layout = {
      title: title,
			geo: {
					scope: 'world',
					resolution: 50,
					showland: true,
					landcolor: '#ffffff',
					//countrycolor: '#000000',
					countrywidth: 1.5,
					subunitcolor: '#888888'
			}
	};

	Plotly.newPlot(id, [data], layout); 
  self.update = function(pts) { // [[lat1, lon1, label1], [lat2, lon2, label2]...]
      data.lat = [];
      data.lon = [];
      data.text = [];
      for (const d of pts) {
        data.lat.push(d[0]);
        data.lon.push(d[1]);
        data.text.push(d[2]);
      }
      Plotly.react(id, [data], layout);
  };
}
