<?php
// o link precisa ser: www.URL.com.br/model.php?key=123456



$link = "https://www.cbf.com.br/futebol-brasileiro/competicoes/campeonato-brasileiro-serie-a/2021";
$url = $link;
$time_array = getdata($url);




function tdrows($elements)
{
    $str = array();
    foreach ($elements as $element) {
        $str[] = trim($element->nodeValue);
    }
    return $str;
}

function getdata($contents)
{

    $ch = curl_init();
    curl_setopt($ch, CURLOPT_URL, $contents);
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
    curl_setopt($ch, CURLOPT_HEADER, 0);
    $content = curl_exec($ch);
    if (curl_errno($ch)) {
        echo curl_error($ch);
    } else {
        $file = 'content.html';

        $fh = fopen($file, 'w');

        if (!$fh) {
            echo "Unable to create $file";
        } else {
            fwrite($fh, $content);


            echo "Saved $file";

            fclose($fh);
        }
    }

    $DOM = new DOMDocument;
    $DOM->loadHTMLFile($file);
    $items = $DOM->getElementsByTagName('tr');
    $return = array();
    foreach ($items as $node) {
        $return[] = tdrows($node->childNodes);
    }
    curl_close($ch);

    return $return;
}



//Retorna os arrays internos na sequencia certa da DOM
function volta($array)
{
    $array[0] = preg_replace('/\s+/', ' ', $array[0]);
    $array[0] = str_replace('0', '', $array[0]);
    $array[0] = str_replace('+1', '', $array[0]);
    $array[0] = str_replace('+2', '', $array[0]);
    $array[0] = str_replace('+3', '', $array[0]);
    $array[0] = str_replace('+4', '', $array[0]);
    $array[0] = str_replace('+5', '', $array[0]);
    $array[0] = str_replace('+6', '', $array[0]);
    $array[0] = str_replace('+7', '', $array[0]);
    $array[0] = str_replace('+8', '', $array[0]);
    $array[0] = str_replace('+9', '', $array[0]);
    $array[0] = str_replace('-1', '', $array[0]);
    $array[0] = str_replace('-2', '', $array[0]);
    $array[0] = str_replace('-3', '', $array[0]);
    $array[0] = str_replace('-4', '', $array[0]);
    $array[0] = str_replace('-5', '', $array[0]);
    $array[0] = str_replace('-6', '', $array[0]);
    $array[0] = str_replace('-7', '', $array[0]);
    $array[0] = str_replace('-8', '', $array[0]);
    $array[0] = str_replace('-9', '', $array[0]);
    $array[0] = str_replace('1º', '', $array[0]);
    $array[0] = str_replace('2º', '', $array[0]);
    $array[0] = str_replace('3º', '', $array[0]);
    $array[0] = str_replace('4º', '', $array[0]);
    $array[0] = str_replace('5º', '', $array[0]);
    $array[0] = str_replace('6º', '', $array[0]);
    $array[0] = str_replace('7º', '', $array[0]);
    $array[0] = str_replace('8º', '', $array[0]);
    $array[0] = str_replace('9º', '', $array[0]);
    $array[0] = str_replace('1', '', $array[0]);
    $array[0] = str_replace('2', '', $array[0]);
    $array[0] = str_replace('3', '', $array[0]);
    $array[0] = str_replace('4', '', $array[0]);
    $array[0] = str_replace('5', '', $array[0]);
    $array[0] = str_replace('6', '', $array[0]);
    $array[0] = str_replace('7', '', $array[0]);
    $array[0] = str_replace('8', '', $array[0]);
    $array[0] = str_replace('9', '', $array[0]);
    $array[0] = preg_replace('/\s+/', ' ', $array[0]);





    $tabela = [
        'Equipe' => $array[0],
        'PTS' => $array[2],
        'J' => $array[4],
        'V' => $array[6],
        'E' => $array[8],
        'D' => $array[10],
        'GP' => $array[12],
        'GC' => $array[14],
        'SG' => $array[16],
        'CA' => $array[18],
        'CV' => $array[20],
        'POR' => $array[22]
        // $array[25],
        // $array[27]
    ];

    return $tabela;
}

//volta os Arrays externos na ordem certa na sequencia da DOM
$time_master =
    array(
        volta($time_array[1]),
        volta($time_array[3]),
        volta($time_array[5]),
        volta($time_array[7]),
        volta($time_array[9]),
        volta($time_array[11]),
        volta($time_array[13]),
        volta($time_array[15]),
        volta($time_array[17]),
        volta($time_array[19]),
        volta($time_array[21]),
        volta($time_array[23]),
        volta($time_array[25]),
        volta($time_array[27]),
        volta($time_array[29]),
        volta($time_array[31]),
        volta($time_array[33]),
        volta($time_array[35]),
        volta($time_array[37]),
        volta($time_array[39])
    );

echo '<pre>';
print_r($time_array);
echo '</pre>';
