<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<item>
			<xsl:value-of select="/root/item">
				<item>value</item>
			</xsl:value-of>
		</item>
	</xsl:template>
</xsl:stylesheet>